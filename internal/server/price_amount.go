package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	priceamountdomain "github.com/smallbiznis/railzway/internal/priceamount/domain"
)

// @Summary      Create Price Amount
// @Description  Create a new price amount
// @Tags         price_amounts
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        request body priceamountdomain.CreateRequest true "Create Price Amount Request"
// @Success      200  {object}  priceamountdomain.PriceAmount
// @Router       /price_amounts [post]
func (s *Server) CreatePriceAmount(c *gin.Context) {
	var req priceamountdomain.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.priceAmountSvc.Create(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil {
		targetID := resp.ID.String()
		metadata := map[string]any{
			"price_amount_id":   resp.ID,
			"price_id":          resp.PriceID,
			"currency":          resp.Currency,
			"unit_amount_cents": resp.UnitAmountCents,
		}
		if resp.MinimumAmountCents != nil {
			metadata["minimum_amount_cents"] = *resp.MinimumAmountCents
		}
		if resp.MaximumAmountCents != nil {
			metadata["maximum_amount_cents"] = *resp.MaximumAmountCents
		}
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "price_amount.create", "price_amount", &targetID, metadata)
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      List Price Amounts
// @Description  List available price amounts
// @Tags         price_amounts
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        price_id query string false "Price ID"
// @Success      200  {object}  []priceamountdomain.PriceAmount
// @Router       /price_amounts [get]
func (s *Server) ListPriceAmounts(c *gin.Context) {

	var req priceamountdomain.ListPriceAmountRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.priceAmountSvc.List(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// @Summary      Get Price Amount
// @Description  Get price amount by ID
// @Tags         price_amounts
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Price Amount ID"
// @Success      200  {object}  priceamountdomain.PriceAmount
// @Router       /price_amounts/{id} [get]
func (s *Server) GetPriceAmountByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))

	resp, err := s.priceAmountSvc.Get(c.Request.Context(), priceamountdomain.GetPriceAmountByID{
		ID: id,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func isPriceAmountValidationError(err error) bool {
	switch err {
	case priceamountdomain.ErrInvalidOrganization,
		priceamountdomain.ErrInvalidPrice,
		priceamountdomain.ErrInvalidCurrency,
		priceamountdomain.ErrInvalidUnitAmount,
		priceamountdomain.ErrInvalidMinAmount,
		priceamountdomain.ErrInvalidMaxAmount,
		priceamountdomain.ErrInvalidMeterID,
		priceamountdomain.ErrInvalidEffectiveFrom,
		priceamountdomain.ErrInvalidEffectiveTo,
		priceamountdomain.ErrEffectiveOverlap,
		priceamountdomain.ErrEffectiveGap,
		priceamountdomain.ErrInvalidID:
		return true
	default:
		return false
	}
}
