package httpclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/httpclient"
)

func TestClient_GetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{Timeout: 5 * time.Second, MaxBodyBytes: 1024, AllowPrivate: true})
	resp, err := c.Get(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "ok", string(body))
}

func TestClient_BlocksLoopbackByDefault(t *testing.T) {
	// httptest.NewServer always binds to 127.0.0.1, which the SSRF
	// dialer must refuse.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{Timeout: 5 * time.Second, MaxBodyBytes: 1024})
	_, err := c.Get(context.Background(), srv.URL, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ssrf")
}

func TestClient_AllowPrivateNetworksLetsLoopbackThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{
		Timeout:      5 * time.Second,
		MaxBodyBytes: 1024,
		AllowPrivate: true,
	})
	resp, err := c.Get(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestClient_BodyCapEnforced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 100)))
	}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{
		Timeout:      5 * time.Second,
		MaxBodyBytes: 10,
		AllowPrivate: true,
	})
	resp, err := c.Get(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	require.Error(t, err, "ReadAll must fail once cap is exceeded")
}

func TestClient_TimeoutHonored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	c := httpclient.NewClient(httpclient.Config{
		Timeout:      200 * time.Millisecond,
		MaxBodyBytes: 1024,
		AllowPrivate: true,
	})
	_, err := c.Get(context.Background(), srv.URL, nil)
	require.Error(t, err)
}
