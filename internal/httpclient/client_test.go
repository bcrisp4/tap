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
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
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

func TestClient_RedirectFromAllowlistedHostToOutsideHostBlocked(t *testing.T) {
	// Reproduces the redirect-bypass concern: when the original
	// hostname is on the suffix allowlist, the client uses the
	// unguarded transport. A redirect to a host outside the allowlist
	// would otherwise reuse that unguarded transport and silently
	// skip the SSRF guard. The CheckRedirect policy must catch it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://evil.example.invalid/", http.StatusFound)
	}))
	defer srv.Close()

	allowlist, err := httpclient.ParseAllowedHosts("127.0.0.1")
	require.NoError(t, err)

	c := httpclient.NewClient(httpclient.Config{
		Timeout:      5 * time.Second,
		MaxBodyBytes: 1024,
		Allowlist:    allowlist,
	})
	_, err = c.Get(context.Background(), srv.URL, nil)
	require.Error(t, err, "redirect to non-allowlisted host must be refused")
	require.Contains(t, err.Error(), "ssrf")
}

func TestClient_RedirectWithinAllowlistedHostAllowed(t *testing.T) {
	// Two hops on the same allowlisted host should succeed.
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, "/end", http.StatusFound)
	})
	mux.HandleFunc("/end", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	allowlist, err := httpclient.ParseAllowedHosts("127.0.0.1")
	require.NoError(t, err)

	c := httpclient.NewClient(httpclient.Config{
		Timeout:      5 * time.Second,
		MaxBodyBytes: 1024,
		Allowlist:    allowlist,
	})
	resp, err := c.Get(context.Background(), srv.URL+"/start", nil)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 2, hits)
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
