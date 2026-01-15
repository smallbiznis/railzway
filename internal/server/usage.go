package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	usagedomain "github.com/smallbiznis/railzway/internal/usage/domain"
)

// @Summary      Ingest Usage
// @Description  Ingest usage event
// @Tags         usage
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        request body usagedomain.CreateIngestRequest true "Ingest Usage Request"
// @Success      200  {object}  usagedomain.Usage
// @Router       /usage [post]
func (s *Server) IngestUsage(c *gin.Context) {

	var req usagedomain.CreateIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, err)
		return
	}
	if meterCode := strings.TrimSpace(req.MeterCode); meterCode != "" {
		c.Set("meter_code", meterCode)
	}

	usage, err := s.usagesvc.Ingest(c.Request.Context(), req)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, usage)
}
