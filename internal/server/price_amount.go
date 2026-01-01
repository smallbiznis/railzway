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
