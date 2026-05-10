package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthz_JSONBody(t *testing.T) {
	t.Parallel()
	mux := NewMux(nil, MuxOpts{StartTime: time.Now(), Version: "test"})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "uptime_seconds")
	assert.Contains(t, body, "db")
	assert.Contains(t, body, "polls_active")
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

func TestMetrics_EnabledReturns200(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	mux := NewMux(d, MuxOpts{MetricsEnabled: true, StartTime: time.Now(), Version: "test"})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/plain")
}

func TestMetrics_DisabledReturns404(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	mux := NewMux(d, MuxOpts{MetricsEnabled: false, StartTime: time.Now(), Version: "test"})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
