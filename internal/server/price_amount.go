package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
)

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
		targetID := resp.ID
		metadata := map[string]any{
			"price_amount_id": resp.ID,
			"price_id":        resp.PriceID,
			"currency":        resp.Currency,
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
