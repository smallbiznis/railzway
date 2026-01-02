package domain

import (
	"context"
	"errors"
	"time"

	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type CreateIngestRequest struct {
	CustomerID string `json:"customer_id" validate:"required,min=1"`
	MeterCode  string `json:"meter_code" validate:"required,min=1"`

	// Usage can be zero or fractional; semantics resolved in rating.
	Value float64 `json:"value" validate:"required"`

	RecordedAt time.Time `json:"recorded_at" validate:"required"`

	// Required; must be non-empty. Uniqueness enforced at DB level.
	IdempotencyKey string `json:"idempotency_key" validate:"required,min=1"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

type ListUsageRequest struct {
	CustomerID     string `json:"customer_id"`
	SubscriptionID string `json:"subscription_id"`
	MeterID        string `json:"meter_id"`
	PageToken      string `json:"page_token"`
	PageSize       int32  `json:"page_size"`
}

type ListUsageResponse struct {
	pagination.PageInfo
	UsageEvents []UsageEvent `json:"usage_events"`
}

type Service interface {
	Ingest(context.Context, CreateIngestRequest) (*UsageEvent, error)
	List(context.Context, ListUsageRequest) (ListUsageResponse, error)
}

var (
	ErrInvalidOrganization     = errors.New("invalid_organization")
	ErrInvalidCustomer         = errors.New("invalid_customer")
	ErrInvalidSubscription     = errors.New("invalid_subscription")
	ErrInvalidSubscriptionItem = errors.New("invalid_subscription_item")
	ErrInvalidMeter            = errors.New("invalid_meter")
	ErrInvalidMeterCode        = errors.New("invalid_meter_code")
	ErrInvalidValue            = errors.New("invalid_value")
	ErrInvalidRecordedAt       = errors.New("invalid_recorded_at")
	ErrInvalidIdempotencyKey   = errors.New("invalid_idempotency_key")
)
