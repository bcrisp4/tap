package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIcons_Get_ServesCachedBytes(t *testing.T) {
	f := newAPIFixture(t)

	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	_, err := f.store.InsertIcon(context.Background(), "abc123", "image/png", pngBytes)
	require.NoError(t, err)

	w := f.do(t, "GET", "/api/v1/icons/abc123", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, `"abc123"`, w.Header().Get("ETag"))
	require.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.Equal(t, pngBytes, w.Body.Bytes())
}

func TestIcons_Get_NotFoundReturns404(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "GET", "/api/v1/icons/missing", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestIcons_Get_IfNoneMatchReturns304(t *testing.T) {
	f := newAPIFixture(t)
	_, err := f.store.InsertIcon(context.Background(), "h0", "image/png", []byte{1, 2, 3})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/icons/h0", nil)
	r.Header.Set("If-None-Match", `"h0"`)
	f.mux.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotModified, w.Code)
	require.Equal(t, `"h0"`, w.Header().Get("ETag"))
	require.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	require.Empty(t, w.Body.Bytes(), "304 must have an empty body")
}

func TestIcons_Get_IfNoneMatchMissOnDifferentHash(t *testing.T) {
	f := newAPIFixture(t)
	_, err := f.store.InsertIcon(context.Background(), "newhash", "image/png", []byte{1})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/icons/newhash", nil)
	r.Header.Set("If-None-Match", `"oldhash"`)
	f.mux.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []byte{1}, w.Body.Bytes())
}
