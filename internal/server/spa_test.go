package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSPA_ServesIndex(t *testing.T) {
	t.Parallel()
	h := SPAHandler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.True(t, strings.HasPrefix(rr.Header().Get("Content-Type"), "text/html"))
}

func TestSPA_FallsBackToIndexForUnknownRoute(t *testing.T) {
	t.Parallel()
	h := SPAHandler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/entry/42", nil))
	require.Equal(t, http.StatusOK, rr.Code, "unknown SPA routes should fall back to index.html")
}
