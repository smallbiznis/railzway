package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pricedomain "github.com/smallbiznis/railzway/internal/price/domain"
)

// @Summary      Create Pricing
// @Description  Create a new pricing
// @Tags         pricings
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        request body pricedomain.CreateRequest true "Create Pricing Request"
// @Success      200  {object}  pricedomain.Pricing
// @Router       /pricings [post]
func (s *Server) CreatePricing(c *gin.Context) {
	var req pricedomain.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.priceSvc.Create(c.Request.Context(), pricedomain.CreateRequest{
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Active:      req.Active,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      List Pricings
// @Description  List available pricings
// @Tags         pricings
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  []pricedomain.Pricing
// @Router       /pricings [get]
func (s *Server) ListPricings(c *gin.Context) {
	resp, err := s.priceSvc.List(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      Get Pricing
// @Description  Get pricing by ID
// @Tags         pricings
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Pricing ID"
// @Success      200  {object}  pricedomain.Pricing
// @Router       /pricings/{id} [get]
func (s *Server) GetPricingByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	resp, err := s.priceSvc.Get(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func isPricingValidationError(err error) bool {
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
