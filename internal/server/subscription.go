package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"github.com/smallbiznis/valora/pkg/db/pagination"
)

func (s *Server) CreateSubscription(c *gin.Context) {
	var req subscriptiondomain.CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.subscriptionSvc.Create(c.Request.Context(), subscriptiondomain.CreateSubscriptionRequest{
		CustomerID:       strings.TrimSpace(req.CustomerID),
		CollectionMode:   req.CollectionMode,
		BillingCycleType: strings.TrimSpace(req.BillingCycleType),
		Items:            normalizeSubscriptionItems(req.Items),
		Metadata:         req.Metadata,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil {
		targetID := resp.ID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "subscription.create", "subscription", &targetID, map[string]any{
			"subscription_id": resp.ID,
			"customer_id":     resp.CustomerID,
			"status":          string(resp.Status),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (s *Server) ListSubscriptions(c *gin.Context) {
	var query struct {
		pagination.Pagination
		Status      string `form:"status"`
		CustomerID  string `form:"customer_id"`
		CreatedFrom string `form:"created_from"`
		CreatedTo   string `form:"created_to"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	createdFrom, err := parseOptionalTime(query.CreatedFrom, false)
	if err != nil {
		AbortWithError(c, newValidationError("created_from", "invalid_created_from", "invalid created_from"))
		return
	}

	createdTo, err := parseOptionalTime(query.CreatedTo, true)
	if err != nil {
		AbortWithError(c, newValidationError("created_to", "invalid_created_to", "invalid created_to"))
		return
	}

	resp, err := s.subscriptionSvc.List(c.Request.Context(), subscriptiondomain.ListSubscriptionRequest{
		Status:      strings.TrimSpace(query.Status),
		CustomerID:  strings.TrimSpace(query.CustomerID),
		PageToken:   query.PageToken,
		PageSize:    int32(query.PageSize),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp.Subscriptions, "page_info": resp.PageInfo})
}

func (s *Server) GetSubscriptionByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := snowflake.ParseString(id); err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	item, err := s.subscriptionSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (s *Server) CancelSubscription(c *gin.Context) {
	s.transitionSubscription(
		c,
		subscriptiondomain.SubscriptionStatusCanceled,
		"subscription.cancel",
	)
}

func (s *Server) ActivateSubscription(c *gin.Context) {
	s.transitionSubscription(
		c,
		subscriptiondomain.SubscriptionStatusActive,
		"subscription.activate",
	)
}

func (s *Server) PauseSubscription(c *gin.Context) {
	s.transitionSubscription(
		c,
		subscriptiondomain.SubscriptionStatusPaused,
		"subscription.pause",
	)
}

func (s *Server) ResumeSubscription(c *gin.Context) {
	s.transitionSubscription(
		c,
		subscriptiondomain.SubscriptionStatusActive,
		"subscription.resume",
	)
}

func (s *Server) transitionSubscription(c *gin.Context, target subscriptiondomain.SubscriptionStatus, auditAction string) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := snowflake.ParseString(id); err != nil {
		AbortWithError(c, newValidationError("id", "invalid_id", "invalid id"))
		return
	}

	if err := s.subscriptionSvc.TransitionSubscription(
		c.Request.Context(),
		id,
		target,
		"",
	); err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil && strings.TrimSpace(auditAction) != "" {
		targetID := id
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, auditAction, "subscription", &targetID, map[string]any{
			"subscription_id": id,
			"status":          string(target),
		})
	}

	c.Status(http.StatusNoContent)
}

func normalizeSubscriptionItems(items []subscriptiondomain.CreateSubscriptionItemRequest) []subscriptiondomain.CreateSubscriptionItemRequest {
	if len(items) == 0 {
		return nil
	}

	normalized := make([]subscriptiondomain.CreateSubscriptionItemRequest, 0, len(items))
	for _, item := range items {
		normalized = append(normalized, subscriptiondomain.CreateSubscriptionItemRequest{
			PriceID:  strings.TrimSpace(item.PriceID),
			MeterID:  strings.TrimSpace(item.MeterID),
			Quantity: item.Quantity,
		})
	}
	return normalized
}

func isSubscriptionValidationError(err error) bool {
	switch {
	case errors.Is(err, subscriptiondomain.ErrInvalidOrganization),
		errors.Is(err, subscriptiondomain.ErrInvalidCustomer),
		errors.Is(err, subscriptiondomain.ErrInvalidSubscription),
		errors.Is(err, subscriptiondomain.ErrInvalidMeterID),
		errors.Is(err, subscriptiondomain.ErrInvalidMeterCode),
		errors.Is(err, subscriptiondomain.ErrInvalidStatus),
		errors.Is(err, subscriptiondomain.ErrInvalidTargetStatus),
		errors.Is(err, subscriptiondomain.ErrInvalidTransition),
		errors.Is(err, subscriptiondomain.ErrMissingSubscriptionItems),
		errors.Is(err, subscriptiondomain.ErrMissingPricing),
		errors.Is(err, subscriptiondomain.ErrMissingCustomer),
		errors.Is(err, subscriptiondomain.ErrBillingCyclesOpen),
		errors.Is(err, subscriptiondomain.ErrInvoicesNotFinalized),
		errors.Is(err, subscriptiondomain.ErrInvalidCollectionMode),
		errors.Is(err, subscriptiondomain.ErrInvalidBillingCycleType),
		errors.Is(err, subscriptiondomain.ErrInvalidStartAt),
		errors.Is(err, subscriptiondomain.ErrInvalidPeriod),
		errors.Is(err, subscriptiondomain.ErrInvalidItems),
		errors.Is(err, subscriptiondomain.ErrInvalidQuantity),
		errors.Is(err, subscriptiondomain.ErrInvalidPrice),
		errors.Is(err, subscriptiondomain.ErrMultipleFlatPrices):
		return true
	default:
		return false
	}
}
