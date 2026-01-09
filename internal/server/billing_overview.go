package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	billingoverviewdomain "github.com/smallbiznis/valora/internal/billingoverview/domain"
)

func (s *Server) GetBillingOverviewMRR(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetMRR(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewRevenue(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetRevenue(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewMRRMovement(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetMRRMovement(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewOutstandingBalance(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetOutstandingBalance(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewCollectionRate(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetCollectionRate(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOverviewSubscribers(c *gin.Context) {
	if s.billingOverviewSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	req, err := parseBillingOverviewRequest(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOverviewSvc.GetSubscribers(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func parseBillingOverviewRequest(c *gin.Context) (billingoverviewdomain.OverviewRequest, error) {
	startValue, err := parseOptionalTime(c.Query("start"), false)
	if err != nil {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("start", "invalid_time", "invalid start time")
	}
	endValue, err := parseOptionalTime(c.Query("end"), true)
	if err != nil {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("end", "invalid_time", "invalid end time")
	}

	granularityValue := strings.ToLower(strings.TrimSpace(c.Query("granularity")))
	if granularityValue == "" {
		granularityValue = string(billingoverviewdomain.GranularityDay)
	}

	var granularity billingoverviewdomain.Granularity
	switch granularityValue {
	case string(billingoverviewdomain.GranularityDay):
		granularity = billingoverviewdomain.GranularityDay
	case string(billingoverviewdomain.GranularityMonth):
		granularity = billingoverviewdomain.GranularityMonth
	default:
		return billingoverviewdomain.OverviewRequest{}, newValidationError("granularity", "invalid_granularity", "invalid granularity")
	}

	compareValue, err := parseOptionalBool(c.Query("compare"))
	if err != nil {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("compare", "invalid_compare", "invalid compare flag")
	}

	now := time.Now().UTC()
	start := now.AddDate(0, 0, -30)
	end := now
	if startValue != nil {
		start = startValue.UTC()
	}
	if endValue != nil {
		end = endValue.UTC()
	}
	if startValue == nil && endValue != nil {
		start = end.AddDate(0, 0, -30)
	}
	if start.After(end) {
		return billingoverviewdomain.OverviewRequest{}, newValidationError("range", "invalid_range", "start must be before end")
	}

	compare := false
	if compareValue != nil {
		compare = *compareValue
	}

	return billingoverviewdomain.OverviewRequest{
		Start:       start,
		End:         end,
		Granularity: granularity,
		Compare:     compare,
	}, nil
}
