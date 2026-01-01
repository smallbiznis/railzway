package domain

import (
	"context"
	"errors"
)

// Service exposes admin billing dashboard data.
type Service interface {
	ListCustomerBalances(ctx context.Context) (CustomerBalancesResponse, error)
	ListBillingCycles(ctx context.Context) (BillingCycleSummaryResponse, error)
	ListBillingActivity(ctx context.Context, limit int) (BillingActivityResponse, error)
}

var (
	ErrInvalidOrganization = errors.New("invalid_organization")
)
