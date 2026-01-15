package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	publicinvoicedomain "github.com/smallbiznis/railzway/internal/publicinvoice/domain"
)

func (s *Server) RegisterPublicRoutes() {
	public := s.engine.Group("/public")
	public.Use(RequestID())

	public.GET("/orgs/:org_id/invoices/:invoice_token", s.GetPublicInvoice)
	public.GET("/orgs/:org_id/invoices/:invoice_token/status", s.GetPublicInvoiceStatus)
	public.POST("/orgs/:org_id/invoices/:invoice_token/checkout-session", s.CreatePublicCheckoutSession)
	public.POST("/orgs/:org_id/invoices/:invoice_token/process-payment", s.ProcessPublicPayment)
	public.GET("/orgs/:org_id/payment_methods", s.GetPublicPaymentMethods)
}

func (s *Server) GetPublicInvoice(c *gin.Context) {
	orgID, token, ok := s.publicInvoiceParams(c)
	if !ok {
		s.respondPublicInvoiceUnavailable(c)
		return
	}
	if !s.publicInvoiceLimiter.Allow(publicInvoiceRateKey(orgID, token, c.ClientIP())) {
		AbortWithError(c, ErrRateLimited)
		return
	}

	resp, err := s.publicInvoiceSvc.GetInvoiceForPublicView(c.Request.Context(), orgID, token)
	if err != nil {
		s.handlePublicInvoiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetPublicInvoiceStatus(c *gin.Context) {
	orgID, token, ok := s.publicInvoiceParams(c)
	if !ok {
		s.respondPublicInvoiceUnavailable(c)
		return
	}
	if !s.publicInvoiceLimiter.Allow(publicInvoiceRateKey(orgID, token, c.ClientIP())) {
		AbortWithError(c, ErrRateLimited)
		return
	}

	status, err := s.publicInvoiceSvc.GetInvoicePublicStatus(c.Request.Context(), orgID, token)
	if err != nil {
		s.handlePublicInvoiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (s *Server) CreatePublicCheckoutSession(c *gin.Context) {
	orgID, token, ok := s.publicInvoiceParams(c)
	if !ok {
		s.respondPublicInvoiceUnavailable(c)
		return
	}
	if !s.publicPaymentIntentLimiter.Allow(publicInvoiceRateKey(orgID, token, c.ClientIP())) {
		AbortWithError(c, ErrRateLimited)
		return
	}

	provider := c.Query("provider")
	// default to stripe for backward compatibility logic if needed, but updated frontend sends provider
	if provider == "" {
		provider = "stripe"
	}

	resp, err := s.publicInvoiceSvc.CreateCheckoutSession(c.Request.Context(), orgID, token, provider)
	if err != nil {
		s.handlePublicInvoiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) ProcessPublicPayment(c *gin.Context) {
	orgID, token, ok := s.publicInvoiceParams(c)
	if !ok {
		s.respondPublicInvoiceUnavailable(c)
		return
	}
	// Reuse limiter or create new one
	if !s.publicPaymentIntentLimiter.Allow(publicInvoiceRateKey(orgID, token, c.ClientIP())) {
		AbortWithError(c, ErrRateLimited)
		return
	}

	var payload map[string]any
	if err := c.BindJSON(&payload); err != nil {
		AbortWithError(c, ErrInvalidRequest)
		return
	}

	provider, _ := payload["provider"].(string)
	if provider == "" {
		AbortWithError(c, ErrInvalidRequest)
		return
	}

	resp, err := s.publicInvoiceSvc.ProcessCheckoutSession(c.Request.Context(), orgID, token, provider, payload)
	if err != nil {
		s.handlePublicInvoiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) GetPublicPaymentMethods(c *gin.Context) {
	orgIDRaw := strings.TrimSpace(c.Param("org_id"))
	if orgIDRaw == "" {
		AbortWithError(c, ErrNotFound)
		return
	}
	orgID, err := snowflake.ParseString(orgIDRaw)
	if err != nil {
		AbortWithError(c, ErrNotFound)
		return
	}
	if !s.publicPaymentMethodsLimiter.Allow(publicPaymentMethodsRateKey(orgID, c.ClientIP())) {
		AbortWithError(c, ErrRateLimited)
		return
	}

	if cached, ok := s.publicPaymentMethodsCache.Get(orgID.String()); ok {
		c.JSON(http.StatusOK, gin.H{"methods": cached})
		return
	}

	methods, err := s.publicInvoiceSvc.ListPaymentMethods(c.Request.Context(), orgID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	s.publicPaymentMethodsCache.Set(orgID.String(), methods)
	c.JSON(http.StatusOK, gin.H{"methods": methods})
}

func (s *Server) publicInvoiceParams(c *gin.Context) (snowflake.ID, string, bool) {
	orgIDRaw := strings.TrimSpace(c.Param("org_id"))
	token := strings.TrimSpace(c.Param("invoice_token"))
	if orgIDRaw == "" || token == "" {
		return 0, "", false
	}
	orgID, err := snowflake.ParseString(orgIDRaw)
	if err != nil {
		return 0, "", false
	}
	return orgID, token, true
}

func publicInvoiceRateKey(orgID snowflake.ID, token string, ip string) string {
	if orgID == 0 || token == "" {
		return ""
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "unknown"
	}
	return orgID.String() + ":" + token + ":" + ip
}

func publicPaymentMethodsRateKey(orgID snowflake.ID, ip string) string {
	if orgID == 0 {
		return ""
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "unknown"
	}
	return orgID.String() + ":" + ip
}

func (s *Server) handlePublicInvoiceError(c *gin.Context, err error) {
	if errors.Is(err, publicinvoicedomain.ErrInvoiceUnavailable) {
		s.respondPublicInvoiceUnavailable(c)
		return
	}
	AbortWithError(c, err)
}

func (s *Server) respondPublicInvoiceUnavailable(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, publicInvoiceUnavailablePayload())
}

func publicInvoiceUnavailablePayload() publicInvoiceErrorResponse {
	return publicInvoiceErrorResponse{
		Code:    "INVOICE_NOT_AVAILABLE",
		Message: "This invoice link is no longer available.",
	}
}

type publicInvoiceErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
