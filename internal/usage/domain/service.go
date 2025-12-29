package domain

import (
	"context"
	"errors"
	"time"

	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type CreateIngestRequest struct {
	CustomerID     string         `json:"customer_id"`
	MeterCode      string         `json:"meter_code"`
	Value          float64        `json:"value"`
	RecordedAt     time.Time      `json:"recorded_at"`
	IdempotencyKey *string        `json:"idempotency_key"`
	Metadata       map[string]any `json:"metadata"`
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
)
