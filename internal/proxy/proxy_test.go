package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/proxy"
)

const probeSecret = "deadbeef"

// newAllowedClient returns an httpclient.Client configured for tests:
// SSRF disabled so an httptest.Server on a loopback address can be
// fetched.
func newAllowedClient(t *testing.T) *httpclient.Client {
	t.Helper()
	return httpclient.NewClient(httpclient.Config{
		Timeout:      2 * time.Second,
		MaxBodyBytes: 1 << 20,
		AllowPrivate: true,
	})
}

func newProxy(t *testing.T, origin string) (*proxy.Proxy, string) {
	t.Helper()
	cache := proxy.NewCache(t.TempDir())
	p := proxy.New(proxy.Config{
		Secret:        probeSecret,
		Client:        newAllowedClient(t),
		Cache:         cache,
		MaxBodyBytes:  1 << 20,
		MaxCacheBytes: 10 << 20,
	})
	tok := proxy.EncodeToken(origin, probeSecret)
	return p, tok
}

func TestProxy_ServesAndCachesHit(t *testing.T) {
	var origins atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		origins.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	defer srv.Close()

	p, tok := newProxy(t, srv.URL+"/img.png")

	// First request: origin fetch + cache.
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Contains(t, w.Header().Get("Cache-Control"), "max-age=")

	// Second request: cache hit; no origin call.
	w2 := httptest.NewRecorder()
	p.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusOK, w2.Code)
	require.EqualValues(t, 1, origins.Load(), "origin must not be hit twice")
}

func TestProxy_IfNoneMatchReturns304(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("hi"))
	}))
	defer srv.Close()
	p, tok := newProxy(t, srv.URL+"/x")

	// Prime the cache.
	w1 := httptest.NewRecorder()
	p.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusOK, w1.Code)
	etag := w1.Header().Get("ETag")
	require.NotEmpty(t, etag)

	// If-None-Match → 304.
	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	r2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	p.ServeHTTP(w2, r2)
	require.Equal(t, http.StatusNotModified, w2.Code)
	body, _ := io.ReadAll(w2.Body)
	require.Empty(t, body)
}

func TestProxy_BadTokenIs400(t *testing.T) {
	p, _ := newProxy(t, "https://x")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/notavalidtoken", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxy_OriginErrorIs404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer srv.Close()
	p, tok := newProxy(t, srv.URL+"/missing")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestProxy_DisallowedMIMEIs415(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte("alert(1)"))
	}))
	defer srv.Close()
	p, tok := newProxy(t, srv.URL+"/bad")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestProxy_CacheFailureIs500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()

	// Point the cache at a path that can't be written to so Cache.Put
	// fails — a regular file masquerading as a cache directory makes
	// MkdirAll fail with ENOTDIR for any 2-char shard subdirectory.
	bogusFile := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(bogusFile, []byte("x"), 0o644))
	cache := proxy.NewCache(bogusFile)

	p := proxy.New(proxy.Config{
		Secret:        probeSecret,
		Client:        newAllowedClient(t),
		Cache:         cache,
		MaxBodyBytes:  1 << 20,
		MaxCacheBytes: 10 << 20,
	})
	tok := proxy.EncodeToken(srv.URL+"/x", probeSecret)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusInternalServerError, w.Code,
		"cache write failures must surface as 500, not 404")
}

func TestProxy_NotModifiedIncludesCacheControl(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()
	p, tok := newProxy(t, srv.URL+"/x")

	// Prime cache.
	w1 := httptest.NewRecorder()
	p.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil))
	require.Equal(t, http.StatusOK, w1.Code)

	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	r2.Header.Set("If-None-Match", `"v1"`)
	w2 := httptest.NewRecorder()
	p.ServeHTTP(w2, r2)
	require.Equal(t, http.StatusNotModified, w2.Code)
	require.Contains(t, w2.Header().Get("Cache-Control"), "max-age=",
		"304 must repeat Cache-Control so clients refresh their freshness window")
}

func TestProxy_EncoderProducesValidTokens(t *testing.T) {
	p, _ := newProxy(t, "https://placeholder")
	enc := p.Encoder()
	url := enc("https://cdn.example.com/img/x.png")
	require.True(t, strings.HasPrefix(url, "/api/v1/proxy/"))

	tok := strings.TrimPrefix(url, "/api/v1/proxy/")
	got, err := proxy.DecodeToken(tok, probeSecret)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/img/x.png", got)
}
