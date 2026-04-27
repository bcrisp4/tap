package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/server"
)

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// listenAddr asks the kernel for a free port, closes the socket, and
// returns "127.0.0.1:<port>". Race-y in theory; fine in tests.
func listenAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// waitReady polls /healthz until the server responds 200 OK or the
// deadline elapses, so tests can assert post-startup behaviour without
// a hard-coded sleep.
func waitReady(t *testing.T, addr string) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 20*time.Millisecond, "server never came up")
}

func TestServer_Healthz(t *testing.T) {
	addr := listenAddr(t)
	srv, err := server.New(addr, newSilentLogger())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	waitReady(t, addr)

	resp, err := http.Get("http://" + addr + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `{"status":"ok"}`, string(body))

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	addr := listenAddr(t)
	srv, err := server.New(addr, newSilentLogger())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	// Wait until the server is actually serving before cancelling, so
	// the test deterministically exercises shutdown rather than a race
	// between cancel() and the listener becoming ready.
	waitReady(t, addr)
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("graceful shutdown did not complete")
	}
}
