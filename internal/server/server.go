// Package server hosts the Tap HTTP server skeleton: routing, listen,
// graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", "addr", s.srv.Addr)
		err := s.srv.ListenAndServe()
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
