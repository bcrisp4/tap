package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware wraps h with an OTel span for inbound requests and injects a
// request_id into the context and response headers.
func Middleware(h http.Handler) http.Handler {
	tracer := otel.Tracer("tap/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()

		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.Pattern)
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.Pattern),
			attribute.String("request_id", requestID),
		)

		ctx = context.WithValue(ctx, requestIDKey{}, requestID)

		rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		w.Header().Set("X-Request-ID", requestID)
		h.ServeHTTP(rr, r.WithContext(ctx))

		span.SetAttributes(attribute.Int("http.status_code", rr.status))
	})
}

type requestIDKey struct{}

// RequestIDFromContext extracts the request_id from ctx.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
