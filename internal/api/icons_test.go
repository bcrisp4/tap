package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Realistic 64-char sha256 hex strings — the only shape the API
// accepts after the format-validation gate.
const (
	hashA = "0000000000000000000000000000000000000000000000000000000000000abc"
	hashB = "111111111111111111111111111111111111111111111111111111111111111b"
	hashC = "2222222222222222222222222222222222222222222222222222222222222222"
)

func TestIcons_Get_ServesCachedBytes(t *testing.T) {
	f := newAPIFixture(t)

	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	_, err := f.store.InsertIcon(context.Background(), hashA, "image/png", pngBytes)
	require.NoError(t, err)

	w := f.do(t, "GET", "/api/v1/icons/"+hashA, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, `"`+hashA+`"`, w.Header().Get("ETag"))
	require.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.Equal(t, pngBytes, w.Body.Bytes())
}

func TestIcons_Get_NotFoundReturns404(t *testing.T) {
	f := newAPIFixture(t)
	// 64 hex chars but no row — must hit the GetIconByHash 404 path,
	// not the format-validation path.
	w := f.do(t, "GET", "/api/v1/icons/"+strings.Repeat("d", 64), "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestIcons_Get_BadHashReturns400(t *testing.T) {
	f := newAPIFixture(t)
	for _, bad := range []string{
		"missing",                     // too short
		strings.Repeat("g", 64),       // non-hex chars
		strings.Repeat("a", 63),       // 63 chars
		strings.Repeat("a", 65),       // 65 chars
		strings.ToUpper(hashA),        // uppercase rejected
	} {
		w := f.do(t, "GET", "/api/v1/icons/"+bad, "")
		require.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for %q", bad)
	}
}

// RFC 7232: a 304 means the resource exists and the client's cached
// copy is fresh. For an unknown hash the existence-check must fire
// first and return 404 — even when the client sends If-None-Match: *.
func TestIcons_Get_IfNoneMatchAgainstMissingHashReturns404(t *testing.T) {
	f := newAPIFixture(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/icons/"+strings.Repeat("e", 64), nil)
	r.Header.Set("If-None-Match", "*")
	f.mux.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestIcons_Get_IfNoneMatchReturns304(t *testing.T) {
	f := newAPIFixture(t)
	_, err := f.store.InsertIcon(context.Background(), hashB, "image/png", []byte{1, 2, 3})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/icons/"+hashB, nil)
	r.Header.Set("If-None-Match", `"`+hashB+`"`)
	f.mux.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotModified, w.Code)
	require.Equal(t, `"`+hashB+`"`, w.Header().Get("ETag"))
	require.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	require.Empty(t, w.Body.Bytes(), "304 must have an empty body")
}

func TestIcons_Get_IfNoneMatchMissOnDifferentHash(t *testing.T) {
	f := newAPIFixture(t)
	_, err := f.store.InsertIcon(context.Background(), hashC, "image/png", []byte{1})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/icons/"+hashC, nil)
	r.Header.Set("If-None-Match", `"`+hashA+`"`)
	f.mux.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []byte{1}, w.Body.Bytes())
}
