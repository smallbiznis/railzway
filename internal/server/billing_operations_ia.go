package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	auditcontext "github.com/smallbiznis/railzway/internal/auditcontext"
	billingoperationsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
)

// IA Endpoints (Task-Centric Views)
// GET /admin/billing-operations/inbox
func (s *Server) GetBillingOperationsInbox(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	req := billingoperationsdomain.InboxRequest{
		Limit: limit,
	}

	resp, err := s.billingOperationsSvc.GetInbox(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /admin/billing-operations/my-work
func (s *Server) GetBillingOperationsMyWork(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	_, userID := auditcontext.ActorFromContext(c.Request.Context())
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	req := billingoperationsdomain.MyWorkRequest{
		Limit: limit,
	}

	resp, err := s.billingOperationsSvc.GetMyWork(c.Request.Context(), userID, req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /admin/billing-operations/recently-resolved
func (s *Server) GetBillingOperationsRecentlyResolved(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	_, userID := auditcontext.ActorFromContext(c.Request.Context())
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	limit, err := parseBillingOperationsLimit(c)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	req := billingoperationsdomain.RecentlyResolvedRequest{
		Limit: limit,
	}

	resp, err := s.billingOperationsSvc.GetRecentlyResolved(c.Request.Context(), userID, req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /admin/billing-operations/team
func (s *Server) GetBillingOperationsTeamView(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	// TODO: Add manager role check
	_, userID := auditcontext.ActorFromContext(c.Request.Context())
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	req := billingoperationsdomain.TeamViewRequest{}

	resp, err := s.billingOperationsSvc.GetTeamView(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /admin/billing-operations/invoices/:id/payments
func (s *Server) GetBillingOperationsInvoicePayments(c *gin.Context) {
	if s.billingOperationsSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	invoiceID := c.Param("id")
	if invoiceID == "" {
		AbortWithError(c, newValidationError("id", "missing_id", "invoice id is required"))
		return
	}

	// TODO: Add proper RBAC here (e.g. valid org member) - currently handled by middleware & service
	resp, err := s.billingOperationsSvc.GetInvoicePayments(c.Request.Context(), invoiceID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
