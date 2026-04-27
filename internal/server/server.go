// Package server hosts the Tap HTTP server skeleton: routing, listen,
// graceful shutdown.
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

// Server wraps a configured *http.Server.
type Server struct {
	logger *slog.Logger
	srv    *http.Server
}

// New builds a Server bound to addr. The HTTP server isn't started
// until Run is called.
func New(addr string, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		return nil, errors.New("server: logger is required")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)

	return &Server{
		logger: logger,
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// Run starts the listener and blocks until ctx is cancelled, then
// performs a graceful shutdown with a 30-second deadline.
//
// The TCP listener is bound synchronously before Run returns to its
// goroutine, so a bind failure (e.g. address in use) surfaces as the
// returned error without ever logging "http server listening".
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.srv.Addr, err)
	}
	s.logger.Info("http server listening", "addr", listener.Addr().String())

	errCh := make(chan error, 1)
	go func() {
		err := s.srv.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	s.logger.Info("http server shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	// Drain ListenAndServe's exit value.
	if err := <-errCh; err != nil {
		return err
	}
	return nil
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
