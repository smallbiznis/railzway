package domain

import (
	"context"
	"errors"
)

type Service interface {
	RunRating(context.Context, string) error
}

var (
	ErrInvalidBillingCycle    = errors.New("invalid_billing_cycle")
	ErrBillingCycleNotFound   = errors.New("billing_cycle_not_found")
	ErrBillingCycleNotClosing = errors.New("billing_cycle_not_closing")
	ErrMissingUsage           = errors.New("missing_usage")
	ErrMissingPriceAmount     = errors.New("missing_price_amount")
	ErrMissingMeter           = errors.New("missing_meter")
	ErrInvalidQuantity        = errors.New("invalid_quantity")
	ErrNoSubscriptionItems    = errors.New("no_subscription_items")
)
