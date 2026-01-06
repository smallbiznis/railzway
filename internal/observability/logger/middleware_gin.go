package logger

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditcontext "github.com/smallbiznis/valora/internal/auditcontext"
	obscontext "github.com/smallbiznis/valora/internal/observability/context"
	"go.uber.org/zap"
)

// MiddlewareConfig controls request logging behavior.
type MiddlewareConfig struct {
	Debug           bool
	ErrorClassifier func(err error) (string, string)
}

// GinMiddleware logs each request with correlation identifiers and safe fields.
func GinMiddleware(cfg MiddlewareConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := ensureRequestID(c)

		ctx := c.Request.Context()
		ctx = obscontext.WithRequestID(ctx, requestID)
		ctx = auditcontext.WithRequestID(ctx, requestID)
		ctx = auditcontext.WithIPAddress(ctx, c.ClientIP())
		ctx = auditcontext.WithUserAgent(ctx, c.Request.UserAgent())
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		route := c.FullPath()
		if strings.TrimSpace(route) == "" {
			route = "unknown"
		}
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("route", route),
			zap.Int("status", status),
			zap.Int64("duration_ms", time.Since(start).Milliseconds()),
			zap.Int64("bytes_in", normalizeBytes(c.Request.ContentLength)),
			zap.Int("bytes_out", normalizeSize(c.Writer.Size())),
		}

		if meterCode := strings.TrimSpace(c.GetString("meter_code")); meterCode != "" {
			fields = append(fields, zap.String("meter_code", meterCode))
		}

		var errorType, errorCode string
		if lastErr := c.Errors.Last(); lastErr != nil {
			if cfg.ErrorClassifier != nil {
				errorType, errorCode = cfg.ErrorClassifier(lastErr.Err)
			}
			fields = append(fields,
				zap.String("error_type", errorType),
				zap.String("error_code", errorCode),
			)
			if cfg.Debug {
				fields = append(fields, zap.Stack("stack"))
			}
		}

		log := FromContext(c.Request.Context())
		logRequest(log, route, status, errorType, fields)
	}
}

func ensureRequestID(c *gin.Context) string {
	requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
	if requestID == "" {
		requestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
	}
	if requestID == "" {
		requestID = strings.TrimSpace(c.GetString("request_id"))
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}

	c.Set("request_id", requestID)
	c.Header("X-Request-Id", requestID)
	return requestID
}

func logRequest(log *zap.Logger, route string, status int, errorType string, fields []zap.Field) {
	if log == nil {
		return
	}

	level := zap.InfoLevel
	if status >= http.StatusInternalServerError {
		level = zap.ErrorLevel
	}
	if isUsageIngest(route) && status >= http.StatusBadRequest && status < http.StatusInternalServerError && errorType == "validation_error" {
		level = zap.DebugLevel
	}

	if isMetric(route) {
		level = zap.DebugLevel
	}

	switch level {
	case zap.DebugLevel:
		log.Debug("http_request", fields...)
	case zap.ErrorLevel:
		log.Error("http_request", fields...)
	default:
		log.Info("http_request", fields...)
	}
}

func isMetric(route string) bool {
	return strings.EqualFold(strings.TrimSpace(route), "/metrics")
}

func isUsageIngest(route string) bool {
	return strings.EqualFold(strings.TrimSpace(route), "/api/usage")
}

func normalizeBytes(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeSize(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
