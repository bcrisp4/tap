package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Addr    string // ":8080" or "127.0.0.1:8080"
	Handler http.Handler
}

type Server struct {
	cfg      Config
	listener net.Listener
	srv      *http.Server
}

func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}
	s.listener = l
	s.srv = &http.Server{
		Handler:           s.cfg.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := s.srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
		}
	}()
	slog.Info("http listening", "addr", l.Addr().String())
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Shutdown gracefully drains in-flight requests with the context's deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}
