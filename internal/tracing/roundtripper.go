package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type tracingRoundTripper struct {
	wrapped http.RoundTripper
}

// NewRoundTripper wraps wrapped with OTel span creation for each outbound
// HTTP request. Only host is recorded (never the full URL) to avoid leaking
// credentials or tracking params into traces.
func NewRoundTripper(wrapped http.RoundTripper) http.RoundTripper {
	return &tracingRoundTripper{wrapped: wrapped}
}

func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := otel.Tracer("tap/http/client")
	ctx, span := tracer.Start(req.Context(), "http.client "+req.Method)
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("net.peer.name", req.URL.Host),
	)

	resp, err := t.wrapped.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	return resp, nil
}
