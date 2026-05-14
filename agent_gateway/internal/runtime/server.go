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

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/bootstrap"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/security"
)

type Server struct {
	cfg       config.Config
	startedAt time.Time
	token     string
	endpoint  Endpoint
	http      *http.Server
	listener  net.Listener
	container *bootstrap.Container
	buildApp  func(context.Context, bootstrap.Options) (*bootstrap.Container, error)
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
		buildApp:  bootstrap.Build,
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

	container, err := s.buildApp(context.Background(), bootstrap.Options{
		Config:    s.cfg,
		Token:     s.token,
		StartedAt: s.startedAt,
	})
	if err != nil {
		_ = listener.Close()
		return err
	}
	s.container = container
	if err := container.Start(context.Background()); err != nil {
		_ = container.Close()
		_ = listener.Close()
		return err
	}

	s.http = &http.Server{
		Handler:           container.Router,
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
	if s.container != nil {
		if err := s.container.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		s.container = nil
	}
	if s.http == nil {
		return shutdownErr
	}
	if err := s.http.Shutdown(ctx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	return shutdownErr
}

func (s *Server) Endpoint() Endpoint {
	return s.endpoint
}

func (s *Server) Token() string {
	return s.token
}
