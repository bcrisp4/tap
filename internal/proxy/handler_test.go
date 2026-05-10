package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/stretchr/testify/require"
)

// PNG fixture — 8-byte signature + minimal IHDR; sufficient for sniff.
var pngFixture = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
}

func newOriginServer(t *testing.T, body []byte, ct string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", ct)
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newHandler(t *testing.T) (*proxy.Signer, http.Handler) {
	t.Helper()
	signer := proxy.NewSigner(testKey)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	h := proxy.NewHandler(signer, cache, http.DefaultClient, 10<<20)
	return signer, h
}

func TestHandler_ColdCacheHappyPath(t *testing.T) {
	t.Parallel()
	origin, hits := newOriginServer(t, pngFixture, "image/png", http.StatusOK)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/img.png")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "image/png", rr.Header().Get("Content-Type"))
	require.Equal(t, strconv.Itoa(len(pngFixture)), rr.Header().Get("Content-Length"))
	require.Equal(t, "public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))

	body, _ := io.ReadAll(rr.Body)
	require.Equal(t, pngFixture, body)
	require.Equal(t, int32(1), atomic.LoadInt32(hits))
}

func TestHandler_WarmCacheNoOriginCall(t *testing.T) {
	t.Parallel()
	origin, hits := newOriginServer(t, pngFixture, "image/png", http.StatusOK)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/img.png")

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
		req.SetPathValue("token", tok)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(hits), "origin should be hit exactly once across 3 requests")
}

func TestHandler_BadTokenReturns404(t *testing.T) {
	t.Parallel()
	_, h := newHandler(t)
	for _, tok := range []string{"", "garbage", "abc.def", strings.Repeat("a", 100) + "." + strings.Repeat("b", 22)} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
		req.SetPathValue("token", tok)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code, "bad token %q should 404", tok)
	}
}

func TestHandler_OriginReturnsHTML_415(t *testing.T) {
	t.Parallel()
	origin, _ := newOriginServer(t, []byte("<html></html>"), "text/html", http.StatusOK)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/x")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
}

func TestHandler_OriginReturns404_PassesThrough(t *testing.T) {
	t.Parallel()
	origin, _ := newOriginServer(t, []byte("not found"), "text/plain", http.StatusNotFound)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/missing")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_OriginOversizeBody_502(t *testing.T) {
	t.Parallel()
	huge := make([]byte, 0, 2_000_000)
	huge = append(huge, pngFixture...)
	for len(huge) < 2_000_000 {
		huge = append(huge, 0x00)
	}
	origin, _ := newOriginServer(t, huge, "image/png", http.StatusOK)

	signer := proxy.NewSigner(testKey)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	h := proxy.NewHandler(signer, cache, http.DefaultClient, 1<<20) // 1 MiB body cap

	tok := signer.Sign(origin.URL + "/big")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestHandler_Origin5xxReturns502(t *testing.T) {
	t.Parallel()
	cases := []int{500, 502, 503, 504}
	for _, status := range cases {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			origin, _ := newOriginServer(t, []byte("upstream broke"), "text/plain", status)
			signer, h := newHandler(t)

			tok := signer.Sign(origin.URL + "/x")
			req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
			req.SetPathValue("token", tok)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadGateway, rr.Code,
				"5xx origin (%d) must collapse to 502", status)
		})
	}
}

func TestNewHandler_PanicsOnNilSigner(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "proxy.NewHandler: signer and cache are required", func() {
		proxy.NewHandler(nil, proxy.NewCache(t.TempDir(), 1<<20), http.DefaultClient, 1<<20)
	})
}

func TestNewHandler_PanicsOnNilCache(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "proxy.NewHandler: signer and cache are required", func() {
		proxy.NewHandler(proxy.NewSigner(testKey), nil, http.DefaultClient, 1<<20)
	})
}

// TestHandler_NeverSendsCredentialsToOrigin pins the M3 proxy's anonymous
// origin-fetch posture: even if the SPA's incoming request carries Cookie
// or Authorization (which it always will for an authenticated reader), the
// proxy must not forward those headers to the origin. Mirrors Miniflux's
// posture and is the M6 spec's stated cross-ecosystem trade-off — the
// regression test locks it in so a future refactor can't accidentally
// introduce credential forwarding.
func TestHandler_NeverSendsCredentialsToOrigin(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngFixture)
	}))
	defer origin.Close()

	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/img.png")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	// Simulate an authenticated SPA reader request — the SPA always carries
	// the session cookie + a CSRF token, and a hostile path could synthesise
	// an Authorization header. Neither must reach the origin.
	req.Header.Set("Cookie", "tap_session=somecookie; csrf=value")
	req.Header.Set("Authorization", "Bearer should-not-leak")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	require.Empty(t, gotCookie, "proxy must not forward Cookie to origin")
	require.Empty(t, gotAuth, "proxy must not forward Authorization to origin")
}

func TestHandler_IncrementsCacheHitMissCounters(t *testing.T) {
	t.Parallel()
	origin, _ := newOriginServer(t, pngFixture, "image/png", http.StatusOK)
	signer := proxy.NewSigner(testKey)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	h := proxy.NewHandler(signer, cache, http.DefaultClient, 10<<20)
	// sharedReg was initialised in TestMain with metrics.InitWithRegistry.
	tok := signer.Sign(origin.URL + "/img.png")

	// First request — cache miss.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Second request — cache hit.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req2.SetPathValue("token", tok)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code)

	gathered, err := sharedReg.Gather()
	require.NoError(t, err)

	var hits, misses float64
	for _, mf := range gathered {
		if mf.GetName() == "tap_proxy_cache_hits_total" {
			for _, m := range mf.GetMetric() {
				hits += m.GetCounter().GetValue()
			}
		}
		if mf.GetName() == "tap_proxy_cache_misses_total" {
			for _, m := range mf.GetMetric() {
				misses += m.GetCounter().GetValue()
			}
		}
	}
	// We assert counters are positive — the exact values include contributions from
	// other parallel tests that also use the cache.
	require.Positive(t, hits, "expected at least 1 cache hit across all tests")
	require.Positive(t, misses, "expected at least 1 cache miss across all tests")
}
