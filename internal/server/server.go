package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/source-asia/backend/internal/config"
)

// Server wraps net/http.Server with graceful shutdown support.
type Server struct {
	httpServer *http.Server
	log        *slog.Logger
	cfg        *config.Config
}

// New creates a Server using the provided config and handler.
func New(cfg *config.Config, handler http.Handler, log *slog.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.HTTPPort),
			Handler:      handler,
			ReadTimeout:  time.Duration(cfg.HTTPReadTimeoutSeconds) * time.Second,
			WriteTimeout: time.Duration(cfg.HTTPWriteTimeoutSeconds) * time.Second,
		},
		log: log,
		cfg: cfg,
	}
}

// Run starts the HTTP listener and blocks until SIGINT or SIGTERM is received.
// It then drains in-flight requests within the shutdown timeout before returning.
func (s *Server) Run() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		s.log.Info("server listening", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		s.log.Info("shutdown signal received", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(s.cfg.HTTPShutdownTimeout)*time.Second)
	defer cancel()

	s.log.Info("shutting down gracefully", "timeout_seconds", s.cfg.HTTPShutdownTimeout)
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	s.log.Info("server exited cleanly")
	return nil
}
