package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	paymentproviderdomain "github.com/smallbiznis/valora/internal/paymentprovider/domain"
)

type upsertPaymentProviderConfigRequest struct {
	Provider string         `json:"provider"`
	Config   map[string]any `json:"config"`
}

type updatePaymentProviderStatusRequest struct {
	IsActive *bool `json:"is_active"`
}

func (s *Server) ListPaymentProviderCatalog(c *gin.Context) {
	resp, err := s.paymentProviderSvc.ListCatalog(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"providers": resp})
}

func (s *Server) ListPaymentProviderConfigs(c *gin.Context) {
	resp, err := s.paymentProviderSvc.ListConfigs(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"configs": resp})
}

func (s *Server) UpsertPaymentProviderConfig(c *gin.Context) {
	var req upsertPaymentProviderConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.paymentProviderSvc.UpsertConfig(c.Request.Context(), paymentproviderdomain.UpsertRequest{
		Provider: strings.TrimSpace(req.Provider),
		Config:   req.Config,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": resp})
}

func (s *Server) UpdatePaymentProviderStatus(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("provider"))
	var req updatePaymentProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}
	if req.IsActive == nil {
		AbortWithError(c, newValidationError("is_active", "invalid_is_active", "invalid is_active"))
		return
	}

	resp, err := s.paymentProviderSvc.SetActive(c.Request.Context(), provider, *req.IsActive)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": resp})
}
