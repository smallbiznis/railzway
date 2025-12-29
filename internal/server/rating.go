package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) RunRatingJob(c *gin.Context) {
	var req struct {
		BillingCycleID string `json:"billing_cycle_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	req.BillingCycleID = strings.TrimSpace(req.BillingCycleID)
	if req.BillingCycleID == "" {
		AbortWithError(c, newValidationError("billing_cycle_id", "invalid_billing_cycle_id", "invalid billing cycle id"))
		return
	}

	if err := s.ratingSvc.RunRating(c.Request.Context(), req.BillingCycleID); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
