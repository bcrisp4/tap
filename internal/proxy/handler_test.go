package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
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
