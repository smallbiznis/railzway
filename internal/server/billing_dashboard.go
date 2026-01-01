package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) ListBillingCustomers(c *gin.Context) {
	if s.billingDashboardSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	resp, err := s.billingDashboardSvc.ListCustomerBalances(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"customers": resp.Customers})
}

func (s *Server) ListBillingCycles(c *gin.Context) {
	if s.billingDashboardSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	resp, err := s.billingDashboardSvc.ListBillingCycles(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"cycles": resp.Cycles})
}

func (s *Server) ListBillingActivity(c *gin.Context) {
	if s.billingDashboardSvc == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	resp, err := s.billingDashboardSvc.ListBillingActivity(c.Request.Context(), 15)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"activity": resp.Activity})
}
