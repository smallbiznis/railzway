package guard

import (
	"errors"
	"strings"
	"time"

	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
)

var (
	ErrSubscriptionNotActive   = errors.New("subscription_not_active")
	ErrMissingActivation       = errors.New("subscription_missing_activation")
	ErrInvalidBillingCycleType = errors.New("invalid_billing_cycle_type")
	ErrCycleNotOpen            = errors.New("billing_cycle_not_open")
	ErrCycleNotReadyToClose    = errors.New("billing_cycle_not_ready_to_close")
)

func EnsureSubscriptionCanOpenBillingCycle(status subscriptiondomain.SubscriptionStatus, activatedAt *time.Time, cycleType string) error {
	if status != subscriptiondomain.SubscriptionStatusActive {
		return ErrSubscriptionNotActive
	}
	if activatedAt == nil {
		return ErrMissingActivation
	}
	if strings.TrimSpace(cycleType) == "" {
		return ErrInvalidBillingCycleType
	}
	return nil
}

func EnsureBillingCycleCanClose(status billingcycledomain.BillingCycleStatus, periodEnd time.Time, now time.Time) error {
	if status != billingcycledomain.BillingCycleStatusOpen {
		return ErrCycleNotOpen
	}
	if now.Before(periodEnd) {
		return ErrCycleNotReadyToClose
	}
	return nil
}
