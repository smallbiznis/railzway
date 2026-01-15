package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// @Summary      List Countries
// @Description  List available countries
// @Tags         reference
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  []referencedomain.Country
// @Router       /countries [get]
func (s *Server) ListCountries(c *gin.Context) {
	countries, err := s.refrepo.ListCountries(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": countries})
}

// @Summary      List Timezones
// @Description  List timezones for a country
// @Tags         reference
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        country  query     string  true  "Country Code"
// @Success      200      {object}  []string
// @Router       /timezones [get]
func (s *Server) ListTimezones(c *gin.Context) {
	country := strings.TrimSpace(c.Query("country"))
	if country == "" {
		AbortWithError(c, newValidationError("country", "invalid_country", "invalid country"))
		return
	}

	timezones, err := s.refrepo.ListTimezonesByCountry(c.Request.Context(), country)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": timezones})
}

// @Summary      List Currencies
// @Description  List available currencies
// @Tags         reference
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  []referencedomain.Currency
// @Router       /currencies [get]
func (s *Server) ListCurrencies(c *gin.Context) {
	currencies, err := s.refrepo.ListCurrencies(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": currencies})
}
