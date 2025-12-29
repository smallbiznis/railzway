package domain

import (
	"context"
	"errors"

	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type ListBillingCycleRequest struct{}

type ListBillingCycleResponse struct {
	pagination.PageInfo
	BillingCycle []BillingCycle `json:"billing_cycles"`
}

type Service interface {
	List(context.Context, ListBillingCycleRequest) (ListBillingCycleResponse, error)
	EnsureBillingCycles(context.Context) error
}

var (
	ErrMultipleOpenCycles = errors.New("multiple_open_cycles")
	ErrInvalidCyclePeriod = errors.New("invalid_cycle_period")
)
