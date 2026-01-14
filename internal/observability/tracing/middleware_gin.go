package tracing

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	obscontext "github.com/smallbiznis/railzway/internal/observability/context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// GinMiddleware instruments inbound HTTP requests.
func GinMiddleware() gin.HandlerFunc {
	tracer := otel.Tracer("railzway/http")
	return func(c *gin.Context) {
		ctx := ExtractContext(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		ctx, span := tracer.Start(ctx, "HTTP "+strings.ToUpper(c.Request.Method), trace.WithSpanKind(trace.SpanKindServer))

		requestID := obscontext.RequestIDFromContext(ctx)
		if requestID != "" {
			member, err := baggage.NewMember("request_id", requestID)
			if err == nil {
				bag, bagErr := baggage.New(member)
				if bagErr == nil {
					ctx = baggage.ContextWithBaggage(ctx, bag)
				}
			}
			span.SetAttributes(attribute.String("request_id", requestID))
		}

		c.Request = c.Request.WithContext(ctx)
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		span.SetName("HTTP " + strings.ToUpper(c.Request.Method) + " " + route)
		span.SetAttributes(SafeAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int64("http.server_duration_ms", time.Since(start).Milliseconds()),
		)...)

		if status := c.Writer.Status(); status >= http.StatusInternalServerError {
			if lastErr := c.Errors.Last(); lastErr != nil {
				if safeErr := SafeError(lastErr.Err); safeErr != nil {
					span.RecordError(safeErr)
				}
			}
			span.SetStatus(codes.Error, "request error")
		}
		span.End()
	}
}
