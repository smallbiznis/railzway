package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallbiznis/railzway/internal/auditcontext"
	billingoperationsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
)

type billingOperationsActionRequest struct {
	ActionType     string         `json:"action_type"`
	EntityType     string         `json:"entity_type"`
	EntityID       string         `json:"entity_id"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type billingOperationsAssignmentRequest struct {
	EntityType           string `json:"entity_type"`
	EntityID             string `json:"entity_id"`
	AssignedTo           string `json:"assigned_to,omitempty"`
	AssignmentTTLMinutes int    `json:"assignment_ttl_minutes,omitempty"`
}

type billingOperationsReleaseRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Reason     string `json:"reason"`
	ReleasedBy string `json:"released_by"`
}

func (s *Server) GetBillingOperationsOverdueInvoices(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOperationsSvc.ListOverdueInvoices(c.Request.Context(), limit)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOperationsOutstandingCustomers(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOperationsSvc.ListOutstandingCustomers(c.Request.Context(), limit)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOperationsPaymentIssues(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOperationsSvc.ListPaymentIssues(c.Request.Context(), limit)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetBillingOperations(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.billingOperationsSvc.GetOperations(c.Request.Context(), limit)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) PostBillingOperationsAction(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req billingOperationsActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.billingOperationsSvc.RecordAction(c.Request.Context(), billingoperationsdomain.RecordActionRequest{
		ActionType:     strings.TrimSpace(req.ActionType),
		EntityType:     strings.TrimSpace(req.EntityType),
		EntityID:       strings.TrimSpace(req.EntityID),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		Metadata:       req.Metadata,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) PostBillingOperationsAssignment(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req billingOperationsAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.billingOperationsSvc.ClaimAssignment(c.Request.Context(), billingoperationsdomain.ClaimAssignmentRequest{
		EntityType:           strings.TrimSpace(req.EntityType),
		EntityID:             strings.TrimSpace(req.EntityID),
		AssignedTo:           strings.TrimSpace(req.AssignedTo),
		AssignmentTTLMinutes: req.AssignmentTTLMinutes,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// func- [x] Backend: Claim & Release with Audit <!-- id: 11 -->
// - [x] Implement Release Assignment (DELETE) Endpoint <!-- id: 7 -->
// - [/] Verify design and integration <!-- id: 6 -->
func (s *Server) ReleaseBillingOperationsAssignment(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req billingOperationsReleaseRequest
	// Try binding JSON, but also support query params if JSON is missing/empty
	if err := c.ShouldBindJSON(&req); err != nil {
		// Ignore error, might be query params only
	}

	if req.EntityType == "" {
		req.EntityType = c.Query("entity_type")
	}
	if req.EntityID == "" {
		req.EntityID = c.Query("entity_id")
	}

	if req.EntityType == "" || req.EntityID == "" {
		AbortWithError(c, newValidationError("entity", "missing_params", "entity_type and entity_id are required"))
		return
	}

	if err := s.billingOperationsSvc.ReleaseAssignment(c.Request.Context(), billingoperationsdomain.ReleaseAssignmentRequest{
		EntityType: strings.TrimSpace(req.EntityType),
		EntityID:   strings.TrimSpace(req.EntityID),
		Reason:     strings.TrimSpace(req.Reason),
		ReleasedBy: strings.TrimSpace(req.ReleasedBy),
	}); err != nil {
		AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// POST /admin/billing-operations/resolve
func (s *Server) ResolveBillingOperationsAssignment(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req billingoperationsdomain.ResolveAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	if err := s.billingOperationsSvc.ResolveAssignment(c.Request.Context(), req); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "resolved"})
}

func parseBillingOperationsLimit(c *gin.Context) (int, error) {
	limitValue, err := parseOptionalInt64(c.Query("limit"))
	if err != nil {
		return 0, newValidationError("limit", "invalid_limit", "invalid limit")
	}
	if limitValue == nil {
		return 0, nil
	}
	if *limitValue < 0 {
		return 0, newValidationError("limit", "invalid_limit", "limit must be positive")
	}
	limit := int(*limitValue)
	if limit > 200 {
		limit = 200
	}
	return limit, nil
}

// GET /finops/performance/me
func (s *Server) GetBillingOperationsPerformanceMe(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	_, userID := auditcontext.ActorFromContext(c.Request.Context())
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var req billingoperationsdomain.GetPerformanceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.billingOperationsSvc.GetMyPerformance(c.Request.Context(), userID, req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /finops/performance/team
func (s *Server) GetBillingOperationsPerformanceTeam(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	// TODO: Replace with real RBAC check for Manager/Lead role
	// For now assume authorized if authenticated, or check a role claim
	_, userID := auditcontext.ActorFromContext(c.Request.Context())
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var req billingoperationsdomain.GetPerformanceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.billingOperationsSvc.GetTeamPerformance(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /finops/exposure-analysis
func (s *Server) GetExposureAnalysis(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req billingoperationsdomain.ExposureAnalysisRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.billingOperationsSvc.GetExposureAnalysis(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
