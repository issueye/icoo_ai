package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connectors/acp"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/security"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

type Server struct {
	cfg       config.Config
	startedAt time.Time
	token     string
	endpoint  Endpoint
	http      *http.Server
	listener  net.Listener
	acpConn   *acp.Connector
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
	}, nil
}

func (s *Server) Start() error {
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
	mux.Handle("/v1/", s.authorize(api.NewRouter(gatewayService)))
	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	dataDir := s.cfg.DataDir
	if dataDir == "" {
		dataDir, err = DefaultDataDir()
		if err != nil {
			_ = listener.Close()
			return err
		}
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

	go func() {
		if err := s.http.Serve(listener); err != nil && err != http.ErrServerClosed {
			// P1 keeps runtime logging out of stdout so future ACP stdio remains clean.
			fmt.Fprintf(os.Stderr, "agent-gateway serve error: %v\n", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.acpConn != nil {
		_ = s.acpConn.Close()
	}
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) newGatewayService() (service.GatewayService, error) {
	if !s.cfg.ACP.Enabled {
		return service.NewMockGatewayService(), nil
	}
	conn, err := acp.NewDefaultConnector(acp.DefaultConnectorOptions{
		Command: s.cfg.ACP.Command,
		Args:    s.cfg.ACP.Args,
		Stderr:  io.Discard,
	})
	if err != nil {
		return nil, err
	}
	s.acpConn = conn
	return service.NewMockGatewayServiceWithAgentsAndStore([]service.AgentProfile{
		{
			ID:          "icoo-ai-acp",
			Name:        "Icoo AI",
			Protocol:    "acp",
			Models:      []string{"mock-gpt"},
			Description: "Default ACP connector profile.",
		},
	}, store.NewMemoryStore()), nil
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
