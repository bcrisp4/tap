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

// Server wraps a configured *http.Server. The mux is held as its own
// field so other packages (the poller's media-proxy mount, the v1 API
// in Plan 08) can attach handlers via Mount without re-asserting the
// http.Handler interface.
type Server struct {
	logger *slog.Logger
	srv    *http.Server
	mux    *http.ServeMux
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
		mux:    mux,
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// Mount attaches handler at pattern. Plan 07 uses this to wire
// /api/v1/proxy/; Plan 08 will mount the rest of the v1 API.
//
// Handlers are typically mounted before Run so the routing table is
// fully assembled before traffic arrives. http.ServeMux is itself
// safe for concurrent registration, so a late Mount won't race, but
// callers should still prefer mounting at startup.
func (s *Server) Mount(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
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
