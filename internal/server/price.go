package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
)

type createPriceRequest struct {
	ProductID            string                      `json:"product_id"`
	Code                 string                      `json:"code"`
	LookupKey            string                      `json:"lookup_key"`
	Name                 string                      `json:"name"`
	Description          string                      `json:"description"`
	PricingModel         pricedomain.PricingModel    `json:"pricing_model"`
	BillingMode          pricedomain.BillingMode     `json:"billing_mode"`
	BillingInterval      pricedomain.BillingInterval `json:"billing_interval"`
	BillingIntervalCount int32                       `json:"billing_interval_count"`
	AggregateUsage       *pricedomain.AggregateUsage `json:"aggregate_usage"`
	BillingUnit          *pricedomain.BillingUnit    `json:"billing_unit"`
	BillingThreshold     *float64                    `json:"billing_threshold"`
	TaxBehavior          pricedomain.TaxBehavior     `json:"tax_behavior"`
	TaxCode              *string                     `json:"tax_code"`
	Version              *int32                      `json:"version"`
	IsDefault            *bool                       `json:"is_default"`
	Active               *bool                       `json:"active"`
	RetiredAt            *time.Time                  `json:"retired_at"`
	Metadata             map[string]any              `json:"metadata"`
}

func (s *Server) CreatePrice(c *gin.Context) {
	var req createPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.priceSvc.Create(c.Request.Context(), pricedomain.CreateRequest{
		ProductID:            strings.TrimSpace(req.ProductID),
		Code:                 strings.TrimSpace(req.Code),
		LookupKey:            req.LookupKey,
		Name:                 req.Name,
		Description:          req.Description,
		PricingModel:         req.PricingModel,
		BillingMode:          req.BillingMode,
		BillingInterval:      req.BillingInterval,
		BillingIntervalCount: req.BillingIntervalCount,
		AggregateUsage:       req.AggregateUsage,
		BillingUnit:          req.BillingUnit,
		BillingThreshold:     req.BillingThreshold,
		TaxBehavior:          req.TaxBehavior,
		TaxCode:              req.TaxCode,
		Version:              req.Version,
		IsDefault:            req.IsDefault,
		Active:               req.Active,
		RetiredAt:            req.RetiredAt,
		Metadata:             req.Metadata,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil {
		targetID := resp.ID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "price.create", "price", &targetID, map[string]any{
			"price_id":      resp.ID,
			"product_id":    resp.ProductID,
			"code":          resp.Code,
			"pricing_model": resp.PricingModel,
			"billing_mode":  resp.BillingMode,
			"active":        resp.Active,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) ListPrices(c *gin.Context) {
	resp, err := s.priceSvc.List(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) GetPriceByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	resp, err := s.priceSvc.Get(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func isPriceValidationError(err error) bool {
	switch err {
	case pricedomain.ErrInvalidOrganization,
		pricedomain.ErrInvalidProduct,
		pricedomain.ErrInvalidCode,
		pricedomain.ErrInvalidPricingModel,
		pricedomain.ErrInvalidBillingMode,
		pricedomain.ErrInvalidBillingInterval,
		pricedomain.ErrInvalidBillingIntervalCount,
		pricedomain.ErrInvalidAggregateUsage,
		pricedomain.ErrInvalidBillingUnit,
		pricedomain.ErrInvalidBillingThreshold,
		pricedomain.ErrInvalidTaxBehavior,
		pricedomain.ErrInvalidVersion,
		pricedomain.ErrInvalidID:
		return true
	default:
		return false
	}
}
