package metrics

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics captures low-cardinality HTTP server metrics.
type HTTPMetrics struct {
	requestDuration metric.Float64Histogram
	inFlight        metric.Int64UpDownCounter
}

// NewHTTPMetrics creates HTTP metrics instruments.
func NewHTTPMetrics(cfg Config, provider metric.MeterProvider) (*HTTPMetrics, error) {
	name := strings.TrimSpace(cfg.ServiceName)
	if name == "" {
		name = "railzway"
	}
	meter := provider.Meter(name + "/http")

	requestDuration, err := meter.Float64Histogram("http.server.duration_ms")
	if err != nil {
		return nil, err
	}
	inFlight, err := meter.Int64UpDownCounter("http.server.in_flight")
	if err != nil {
		return nil, err
	}

	return &HTTPMetrics{
		requestDuration: requestDuration,
		inFlight:        inFlight,
	}, nil
}

// GinMiddleware records request duration and in-flight metrics.
func GinMiddleware(m *HTTPMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil {
			c.Next()
			return
		}
		endpoint := normalizeEndpoint(c.FullPath())
		ctx := c.Request.Context()
		m.inFlight.Add(ctx, 1, metric.WithAttributes(FilterAttributes(attribute.String("endpoint", endpoint))...))
		start := time.Now()
		c.Next()
		m.inFlight.Add(ctx, -1, metric.WithAttributes(FilterAttributes(attribute.String("endpoint", endpoint))...))

		status := c.Writer.Status()
		attrs := FilterAttributes(
			attribute.String("endpoint", endpoint),
			attribute.String("status_code", strconv.Itoa(status)),
		)
		m.requestDuration.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
	}
}

// RecordRequest allows manual recording of HTTP metrics.
func (m *HTTPMetrics) RecordRequest(ctx context.Context, endpoint string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	attrs := FilterAttributes(
		attribute.String("endpoint", normalizeEndpoint(endpoint)),
		attribute.String("status_code", strconv.Itoa(status)),
	)
	m.requestDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "unknown"
	}
	return endpoint
}
