package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	t.Parallel()
	mux := NewMux(nil, MuxOpts{}) // nil DB OK — healthz doesn't touch it
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "ok", rr.Body.String())
}

// Without a DB the mux must degenerate to /healthz only — POST /sessions
// would panic in loginHandler if it were registered with a nil dep.d.
func TestNewMuxNilDBOmitsAuthRoutes(t *testing.T) {
	t.Parallel()
	mux := NewMux(nil, MuxOpts{})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/sessions", nil))
	require.Equal(t, http.StatusNotFound, rr.Code)
}
