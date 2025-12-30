// internal/server/dev_billing.go
package server

import (
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	schedulertesting "github.com/smallbiznis/valora/internal/scheduler/testing"
)

// RegisterDevBillingRoutes adds development-only billing endpoints
func (s *Server) RegisterDevBillingRoutes() {
	if s.cfg.Environment == "production" {
		return
	}

	dev := s.engine.Group("/dev/billing")
	dev.Use(RequestID())
	// dev.Use(s.AuthRequired())

	// Fast-forward billing cycles
	dev.POST("/cycles/:id/fast-forward", s.DevFastForwardCycle)
	dev.POST("/cycles/fast-forward-all", s.DevFastForwardAllCycles)
	dev.POST("/subscriptions/:id/fast-forward-cycle", s.DevFastForwardSubscriptionCycle)

	// Cycle info and debugging
	dev.GET("/cycles/:id/info", s.DevGetCycleInfo)
	dev.GET("/cycles/open", s.DevGetAllOpenCycles)

	// Manual trigger scheduler jobs
	dev.POST("/scheduler/run-once", s.DevRunSchedulerOnce)
	dev.POST("/scheduler/ensure-cycles", s.DevEnsureBillingCycles)
	dev.POST("/scheduler/close-cycles", s.DevCloseCycles)
	dev.POST("/scheduler/rating", s.DevRunRating)
	dev.POST("/scheduler/invoicing", s.DevRunInvoicing)
	dev.POST("/scheduler/recovery", s.DevRunRecovery)

	// Reset and cleanup
	dev.POST("/cycles/:id/reset-errors", s.DevResetCycleErrors)
	dev.POST("/cycles/:id/force-reopen", s.DevForceReopenCycle)
}

func (s *Server) DevFastForwardCycle(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	cycleID, err := snowflake.ParseString(id)
	if err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	helper := schedulertesting.NewTimeAccelerator(s.db)
	if err := helper.FastForwardCycle(c.Request.Context(), cycleID); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "cycle fast-forwarded",
		"cycle_id": id,
	})
}

func (s *Server) DevFastForwardAllCycles(c *gin.Context) {
	helper := schedulertesting.NewTimeAccelerator(s.db)
	affected, err := helper.FastForwardAllOpenCycles(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "all open cycles fast-forwarded",
		"affected_cycles": affected,
	})
}

func (s *Server) DevFastForwardSubscriptionCycle(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	subscriptionID, err := snowflake.ParseString(id)
	if err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	helper := schedulertesting.NewTimeAccelerator(s.db)
	if err := helper.FastForwardSubscriptionCycle(c.Request.Context(), subscriptionID); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "subscription cycle fast-forwarded",
		"subscription_id": id,
	})
}

func (s *Server) DevGetCycleInfo(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	cycleID, err := snowflake.ParseString(id)
	if err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	helper := schedulertesting.NewTimeAccelerator(s.db)
	info, err := helper.GetCycleInfo(c.Request.Context(), cycleID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":                     info.ID.String(),
			"status":                 info.Status,
			"period_start":           info.PeriodStart,
			"period_end":             info.PeriodEnd,
			"time_until_end_seconds": info.TimeUntilEnd.Seconds(),
			"can_close":              info.CanClose,
		},
	})
}

func (s *Server) DevGetAllOpenCycles(c *gin.Context) {
	helper := schedulertesting.NewTimeAccelerator(s.db)
	cycles, err := helper.GetAllOpenCycles(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	data := make([]gin.H, 0, len(cycles))
	for _, cycle := range cycles {
		data = append(data, gin.H{
			"id":                     cycle.ID.String(),
			"status":                 cycle.Status,
			"period_start":           cycle.PeriodStart,
			"period_end":             cycle.PeriodEnd,
			"time_until_end_seconds": cycle.TimeUntilEnd.Seconds(),
			"can_close":              cycle.CanClose,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (s *Server) DevRunSchedulerOnce(c *gin.Context) {
	if s.scheduler == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	if err := s.scheduler.RunOnce(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "scheduler run completed with errors",
			"errors":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "scheduler run completed successfully",
	})
}

func (s *Server) DevEnsureBillingCycles(c *gin.Context) {
	if s.scheduler == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	if err := s.scheduler.EnsureBillingCyclesJob(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "ensure cycles job completed with errors",
			"errors":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ensure cycles job completed successfully",
	})
}

func (s *Server) DevCloseCycles(c *gin.Context) {
	if s.scheduler == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	if err := s.scheduler.CloseCyclesJob(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "close cycles job completed with errors",
			"errors":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "close cycles job completed successfully",
	})
}

func (s *Server) DevRunRating(c *gin.Context) {
	if s.scheduler == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	if err := s.scheduler.RatingJob(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "rating job completed with errors",
			"errors":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "rating job completed successfully",
	})
}

func (s *Server) DevRunInvoicing(c *gin.Context) {
	if s.scheduler == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	if err := s.scheduler.InvoiceJob(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "invoicing job completed with errors",
			"errors":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "invoicing job completed successfully",
	})
}

func (s *Server) DevRunRecovery(c *gin.Context) {
	if s.scheduler == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	if err := s.scheduler.RecoverySweepJob(c.Request.Context()); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "recovery job completed with errors",
			"errors":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "recovery job completed successfully",
	})
}

func (s *Server) DevResetCycleErrors(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	cycleID, err := snowflake.ParseString(id)
	if err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	helper := schedulertesting.NewTimeAccelerator(s.db)
	if err := helper.ResetCycleErrors(c.Request.Context(), cycleID); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "cycle errors reset",
		"cycle_id": id,
	})
}

func (s *Server) DevForceReopenCycle(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	cycleID, err := snowflake.ParseString(id)
	if err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	helper := schedulertesting.NewTimeAccelerator(s.db)
	if err := helper.ForceReopen(c.Request.Context(), cycleID); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "cycle force reopened (DANGER: testing only!)",
		"cycle_id": id,
	})
}
