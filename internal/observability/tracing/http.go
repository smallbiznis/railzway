package tracing

import (
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// WrapHTTPClient instruments an http.Client with tracing propagation.
func WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	clone := *client
	base := clone.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone.Transport = &transport{base: base, tracer: otel.Tracer("valora/http")}
	return &clone
}

type transport struct {
	base   http.RoundTripper
	tracer trace.Tracer
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return t.base.RoundTrip(req)
	}
	ctx, span := t.tracer.Start(req.Context(), "HTTP "+strings.ToUpper(req.Method), trace.WithSpanKind(trace.SpanKindClient))
	carrier := propagation.HeaderCarrier(req.Header)
	InjectContext(ctx, carrier)

	start := time.Now()
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		if safeErr := SafeError(err); safeErr != nil {
			span.RecordError(safeErr)
		}
		span.SetStatus(codes.Error, "client error")
		span.End()
		return resp, err
	}

	route := req.URL.Path
	span.SetName("HTTP " + strings.ToUpper(req.Method) + " " + route)
	span.SetAttributes(SafeAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.host", req.URL.Host),
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.Int64("http.client_duration_ms", time.Since(start).Milliseconds()),
	)...)

	if resp.StatusCode >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, "server error")
	}
	span.End()
	return resp, err
}
