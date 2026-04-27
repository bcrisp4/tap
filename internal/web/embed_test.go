package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandler_RejectsAPIPaths(t *testing.T) {
	h := Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/feeds", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_PlaceholderWhenNoSPA(t *testing.T) {
	if hasSPA() {
		t.Skip("SPA is embedded (-tags embed_spa); placeholder test does not apply")
	}
	h := Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "tap (api only)")
}
