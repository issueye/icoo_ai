package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connectors/acp"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/security"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type Server struct {
	cfg                   config.Config
	startedAt             time.Time
	token                 string
	endpoint              Endpoint
	http                  *http.Server
	listener              net.Listener
	store                 store.Store
	eventBus              *events.Bus
	projector             *eventProjector
	gatewayServiceFactory func(store.Store) (service.GatewayService, error)
	gatewayServiceCloser  io.Closer
}

func NewServer(cfg config.Config) (*Server, error) {
	if cfg.Version == "" {
		cfg.Version = config.Version
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	token, err := security.GenerateToken()
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:       cfg,
		startedAt: time.Now(),
		token:     token,
		eventBus:  events.DefaultBus(),
	}, nil
}

func (s *Server) Start() error {
	dataDir := s.cfg.DataDir
	if dataDir == "" {
		var err error
		dataDir, err = DefaultDataDir()
		if err != nil {
			return err
		}
		s.cfg.DataDir = dataDir
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener

	mux := http.NewServeMux()
	mux.Handle("/health", api.HealthHandler(s.cfg.Version, s.startedAt))
	gatewayService, err := s.newGatewayService()
	if err != nil {
		_ = listener.Close()
		return err
	}
	if closer, ok := gatewayService.(io.Closer); ok {
		s.gatewayServiceCloser = closer
	}
	mux.Handle("/v1/", s.authorize(api.NewRouter(gatewayService)))
	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	endpoint := Endpoint{
		PID:       os.Getpid(),
		BaseURL:   "http://" + listener.Addr().String(),
		StartedAt: s.startedAt,
	}
	endpoint, err = WriteRuntimeFiles(dataDir, endpoint, s.token)
	if err != nil {
		_ = listener.Close()
		return err
	}
	s.endpoint = endpoint
	if s.store != nil {
		s.projector = startEventProjector(s.eventBus, s.store)
	}

	go func() {
		if err := s.http.Serve(listener); err != nil && err != http.ErrServerClosed {
			// P1 keeps runtime logging out of stdout so future ACP stdio remains clean.
			fmt.Fprintf(os.Stderr, "agent-gateway serve error: %v\n", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error
	if s.projector != nil {
		s.projector.Stop()
	}
	if s.gatewayServiceCloser != nil {
		if err := s.gatewayServiceCloser.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		s.gatewayServiceCloser = nil
	}
	if s.http == nil {
		return shutdownErr
	}
	if err := s.http.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	return shutdownErr
}

func (s *Server) newGatewayService() (service.GatewayService, error) {
	memStore := store.NewMemoryStore()
	s.store = memStore
	if s.gatewayServiceFactory != nil {
		return s.gatewayServiceFactory(memStore)
	}
	settingsStore, err := service.NewSQLiteManagementSettingsStore(filepath.Join(s.cfg.DataDir, "management.db"))
	if err != nil {
		return nil, err
	}
	defaultAgents := []service.AgentProfile{
		{ID: "icoo-ai-acp", Name: "Icoo AI", Protocol: "icoo_acp", Models: []string{"gpt-5.4"}, Description: "Icoo ACP agent profile."},
		{ID: "agent-acp", Name: "Agent ACP", Protocol: "agent_acp", Models: []string{"gpt-5.4"}, Description: "Generic ACP agent profile."},
	}

	if !s.cfg.ACP.Enabled {
		return service.NewMockGatewayServiceWithAgentsStoreAndSettingsStore(defaultAgents, memStore, settingsStore), nil
	}

	lazy := newLazyConnector(func() (connector.AgentConnector, error) {
		return s.newACPConnector(memStore)
	}, connector.InitializeRequest{
		ClientName:    "agent-gateway",
		ClientVersion: s.cfg.Version,
	})
	return service.NewConnectorGatewayServiceWithAgentsStoreAndSettingsStore(defaultAgents, memStore, settingsStore, lazy), nil
}

func (s *Server) newACPConnector(memStore store.Store) (connector.AgentConnector, error) {
	poolSize := s.cfg.ACP.PoolSize
	if poolSize <= 0 {
		poolSize = 1
	}

	backends := make([]connector.AgentConnector, 0, poolSize)
	for i := 0; i < poolSize; i++ {
		conn, err := acp.NewDefaultConnector(acp.DefaultConnectorOptions{
			Command: s.cfg.ACP.Command,
			Args:    s.cfg.ACP.Args,
			Stderr: acp.NewStderrAuditSink(acp.StderrAuditSinkOptions{
				Store:   memStore,
				AgentID: "icoo-ai-acp",
			}),
		})
		if err != nil {
			for _, backend := range backends {
				_ = backend.Close()
			}
			return nil, err
		}
		backends = append(backends, conn)
	}

	pool, err := acp.NewPool(backends)
	if err != nil {
		for _, backend := range backends {
			_ = backend.Close()
		}
		return nil, err
	}
	return pool, nil
}

func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := security.BearerToken(r.Header.Get("Authorization"))
		if err != nil || token != s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Endpoint() Endpoint {
	return s.endpoint
}

func (s *Server) Token() string {
	return s.token
}
