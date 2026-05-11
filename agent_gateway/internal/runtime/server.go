package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	gatewayapp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/app"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/security"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type Server struct {
	cfg        config.Config
	startedAt  time.Time
	token      string
	endpoint   Endpoint
	http       *http.Server
	listener   net.Listener
	store      store.Store
	eventBus   *events.Bus
	projector  *eventProjector
	components *gatewayapp.Components
	buildApp   func(context.Context, gatewayapp.BuildOptions) (gatewayapp.Components, error)
}

func NewServer(cfg config.Config) (*Server, error) {
	if cfg.Version == "" {
		cfg.Version = config.Version
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	token := strings.TrimSpace(cfg.AuthToken)
	if token == "" {
		generated, err := security.GenerateToken()
		if err != nil {
			return nil, err
		}
		token = generated
	}
	return &Server{
		cfg:       cfg,
		startedAt: time.Now(),
		token:     token,
		eventBus:  events.DefaultBus(),
		buildApp:  gatewayapp.Build,
	}, nil
}

func (s *Server) Start() error {
	dataDir := s.cfg.DataDir
	if dataDir == "" {
		return fmt.Errorf("data_dir is required")
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener

	components, err := s.buildApp(context.Background(), gatewayapp.BuildOptions{
		Config:   s.cfg,
		Token:    s.token,
		Now:      s.startedAt,
		EventBus: s.eventBus,
	})
	if err != nil {
		_ = listener.Close()
		return err
	}
	s.components = &components
	s.store = components.ConversationStore

	mux := http.NewServeMux()
	mux.Handle("/health", components.HealthHandler)
	mux.Handle("/v1/", s.authorize(components.Router))
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
	if s.components != nil {
		if err := s.components.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		s.components = nil
	}
	if s.http == nil {
		return shutdownErr
	}
	if err := s.http.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	return shutdownErr
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
