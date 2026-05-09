package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServer_StartAndShutdown(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })

	s := New(Config{Addr: "127.0.0.1:0", Handler: mux})
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	resp, err := http.Get("http://" + s.Addr() + "/x")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
}
