// Package server owns the http server
package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server dependencies
type Server struct {
	logger     *slog.Logger
	httpServer *http.Server
	ctx        context.Context
	cancel     context.CancelFunc
}

// New constructs the server
func New(logger *slog.Logger, mux *http.ServeMux) *Server {
	s := &Server{
		logger: logger,
		httpServer: &http.Server{
			Addr:              ":8080",
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.httpServer.BaseContext = func(_ net.Listener) context.Context {
		return s.ctx
	}
	return s
}

// Run starts the server
func (s *Server) Run() error {
	s.logger.Info("server starting")
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown shuts the server down gracefully
func (s *Server) Shutdown(d time.Duration) error {
	s.logger.Info("server shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	err := s.httpServer.Shutdown(ctx)
	s.cancel()
	return err
}
