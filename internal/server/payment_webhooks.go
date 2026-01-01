package server

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
)

func (s *Server) HandlePaymentWebhook(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("provider"))
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	err = s.paymentSvc.IngestWebhook(c.Request.Context(), provider, payload, c.Request.Header)
	if err != nil {
		if errors.Is(err, paymentdomain.ErrEventAlreadyProcessed) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
