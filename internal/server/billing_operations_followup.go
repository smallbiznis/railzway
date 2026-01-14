package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	billingoperationsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
)

// POST /admin/billing-operations/record-follow-up
func (s *Server) RecordBillingOperationsFollowUp(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req struct {
		AssignmentID  string `json:"assignment_id"`
		EmailProvider string `json:"email_provider"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.AssignmentID) == "" {
		AbortWithError(c, newValidationError("assignment_id", "required", "assignment_id is required"))
		return
	}

	if err := s.billingOperationsSvc.RecordFollowUp(c.Request.Context(), billingoperationsdomain.RecordFollowUpRequest{
		AssignmentID:  strings.TrimSpace(req.AssignmentID),
		EmailProvider: strings.TrimSpace(req.EmailProvider),
	}); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "recorded",
		"message": "Follow-up action recorded successfully",
	})
}
