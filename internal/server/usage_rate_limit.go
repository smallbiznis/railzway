package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallbiznis/railzway/internal/observability/logger"
	obsmetrics "github.com/smallbiznis/railzway/internal/observability/metrics"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"github.com/smallbiznis/railzway/internal/ratelimit"
	"go.uber.org/zap"
)

const (
	rateLimitReasonOrgRate                  = "org-rate"
	rateLimitReasonEndpointRate             = "endpoint-rate"
	rateLimitReasonCustomerMeterConcurrency = "customer-meter-concurrency"
)

type usageIngestRateLimitKey struct {
	CustomerID string `json:"customer_id"`
	MeterCode  string `json:"meter_code"`
}

func (s *Server) UsageIngestRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.usageLimiter == nil || !s.usageLimiter.Enabled() {
			c.Next()
			return
		}

		orgID, ok := orgcontext.OrgIDFromContext(c.Request.Context())
		if !ok || orgID == 0 {
			AbortWithError(c, ErrOrgRequired)
			return
		}

		endpoint := normalizeRateLimitEndpoint(c)
		ctx := c.Request.Context()

		allowedRes, err := s.usageLimiter.AllowOrg(ctx, orgID.String())
		if err != nil {
			logger.FromContext(ctx).Warn("usage ingest org rate limit check failed", zap.Error(err))
			AbortWithError(c, ErrServiceUnavailable)
			return
		}
		if !allowedRes.Allowed {
			denyUsageIngestRateLimit(c, endpoint, orgID.String(), rateLimitReasonOrgRate, s.obsMetrics, allowedRes)
			return
		}

		allowedRes, err = s.usageLimiter.AllowEndpoint(ctx, orgID.String())
		if err != nil {
			logger.FromContext(ctx).Warn("usage ingest endpoint rate limit check failed", zap.Error(err))
			AbortWithError(c, ErrServiceUnavailable)
			return
		}
		if !allowedRes.Allowed {
			denyUsageIngestRateLimit(c, endpoint, orgID.String(), rateLimitReasonEndpointRate, s.obsMetrics, allowedRes)
			return
		}

		customerID, meterCode, err := readUsageIngestKey(c)
		if err != nil {
			logger.FromContext(ctx).Warn("usage ingest rate limit read body failed", zap.Error(err))
			AbortWithError(c, invalidRequestError())
			return
		}

		if customerID != "" && meterCode != "" {
			lockToken, allowed, err := s.usageLimiter.TryLockCustomerMeter(ctx, orgID.String(), customerID, meterCode)
			if err != nil {
				logger.FromContext(ctx).Warn("usage ingest concurrency lock failed", zap.Error(err))
				AbortWithError(c, ErrServiceUnavailable)
				return
			}
			if !allowed {
				// Concurrency limit doesn't return detailed stats yet, so we use nil or a dummy result
				denyUsageIngestRateLimit(c, endpoint, orgID.String(), rateLimitReasonCustomerMeterConcurrency, s.obsMetrics, nil)
				return
			}
			defer func() {
				if err := s.usageLimiter.ReleaseCustomerMeter(ctx, orgID.String(), customerID, meterCode, lockToken); err != nil {
					logger.FromContext(ctx).Warn("usage ingest concurrency unlock failed", zap.Error(err))
				}
			}()
		}

		recordRateLimitAllowed(ctx, endpoint, orgID.String(), s.obsMetrics)
		c.Next()
	}
}

type usageLimitErrorResponse struct {
	Error           string `json:"error"`
	Resource        string `json:"resource"`
	Current         int    `json:"current"`
	Limit           int    `json:"limit"`
	Period          string `json:"period"`
	NextReset       string `json:"next_reset"`
	UpgradeRequired bool   `json:"upgrade_required"`
}

func denyUsageIngestRateLimit(c *gin.Context, endpoint, orgID, reason string, metrics *obsmetrics.Metrics, result *ratelimit.RateLimitResult) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	log.Warn("usage ingest rate limit exceeded",
		zap.String("reason", reason),
		zap.String("endpoint", endpoint),
	)
	recordRateLimitDenied(ctx, endpoint, orgID, reason, metrics)

	c.Header("Retry-After", "1")
	c.Header("X-Rate-Limited-Reason", reason)

	if result != nil {
		c.JSON(http.StatusTooManyRequests, usageLimitErrorResponse{
			Error:           "usage_limit_reached",
			Resource:        "usage_ingest",
			Current:         result.Limit - result.Remaining,
			Limit:           result.Limit,
			Period:          "monthly", // Hardcoded for matching behavior as per requirement
			NextReset:       result.ResetTime.Format("2006-01-01"),
			UpgradeRequired: true,
		})
		c.Abort()
		return
	}

	AbortWithError(c, ErrRateLimited)
}

func recordRateLimitAllowed(ctx context.Context, endpoint, orgID string, metrics *obsmetrics.Metrics) {
	if metrics == nil {
		return
	}
	metrics.RecordRateLimitAllowed(ctx, orgID, endpoint)
}

func recordRateLimitDenied(ctx context.Context, endpoint, orgID, reason string, metrics *obsmetrics.Metrics) {
	if metrics == nil {
		return
	}
	metrics.RecordRateLimitDenied(ctx, orgID, endpoint, reason)
}

func readUsageIngestKey(c *gin.Context) (string, string, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", "", err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	if len(body) == 0 {
		return "", "", nil
	}

	var payload usageIngestRateLimitKey
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", nil
	}

	return strings.TrimSpace(payload.CustomerID), strings.TrimSpace(payload.MeterCode), nil
}

func normalizeRateLimitEndpoint(c *gin.Context) string {
	if c == nil {
		return "unknown"
	}
	endpoint := strings.TrimSpace(c.FullPath())
	if endpoint == "" {
		endpoint = strings.TrimSpace(c.Request.URL.Path)
	}
	if endpoint == "" {
		endpoint = "unknown"
	}
	return endpoint
}
