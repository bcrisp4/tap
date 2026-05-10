package tracing_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bcrisp4/tap/internal/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_SetsRequestIDHeader(t *testing.T) {
	_ = tracing.Init(tracing.Opts{})
	defer tracing.Shutdown(nil)

	handler := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/entries", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")
	require.NotEmpty(t, id)
	assert.Len(t, id, 32, "X-Request-ID should be 16 hex bytes = 32 chars")
}

func TestMiddleware_RequestIDInContext(t *testing.T) {
	_ = tracing.Init(tracing.Opts{})
	defer tracing.Shutdown(nil)

	var ctxID string
	handler := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = tracing.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	headerID := rr.Header().Get("X-Request-ID")
	assert.Equal(t, headerID, ctxID, "request_id in context should match header")
}
