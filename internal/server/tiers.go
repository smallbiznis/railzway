package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pricetierdomain "github.com/smallbiznis/railzway/internal/pricetier/domain"
)

// @Summary      List Price Tiers
// @Description  List available price tiers
// @Tags         price_tiers
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  []pricetierdomain.PriceTier
// @Router       /price_tiers [get]
func (s *Server) ListPriceTiers(c *gin.Context) {
	resp, err := s.priceTierSvc.List(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      Get Price Tier
// @Description  Get price tier by ID
// @Tags         price_tiers
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Price Tier ID"
// @Success      200  {object}  pricetierdomain.PriceTier
// @Router       /price_tiers/{id} [get]
func (s *Server) GetPriceTierByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	resp, err := s.priceTierSvc.Get(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      Create Price Tier
// @Description  Create a new price tier
// @Tags         price_tiers
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        request body pricetierdomain.CreateRequest true "Create Price Tier Request"
// @Success      200  {object}  pricetierdomain.PriceTier
// @Router       /price_tiers [post]
func (s *Server) CreatePriceTier(c *gin.Context) {
	var req pricetierdomain.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	req.PriceID = strings.TrimSpace(req.PriceID)
	req.Unit = strings.TrimSpace(req.Unit)

	resp, err := s.priceTierSvc.Create(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func isPriceTierValidationError(err error) bool {
	switch err {
	case pricetierdomain.ErrInvalidOrganization,
		pricetierdomain.ErrInvalidPrice,
		pricetierdomain.ErrInvalidTierMode,
		pricetierdomain.ErrInvalidStartQty,
		pricetierdomain.ErrInvalidEndQty,
		pricetierdomain.ErrInvalidUnitAmount,
		pricetierdomain.ErrInvalidFlatAmount,
		pricetierdomain.ErrInvalidUnit,
		pricetierdomain.ErrInvalidID:
		return true
	default:
		return false
	}
}
