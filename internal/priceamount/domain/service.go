package domain

import (
	"context"
	"errors"
	"time"

	"github.com/bwmarrin/snowflake"
)

type ListPriceAmountRequest struct {
	PriceID       string     `url:"price_id" form:"price_id"`
	EffectiveFrom *time.Time `url:"effective_from" form:"effective_from"`
	EffectiveTo   *time.Time `url:"effective_to" form:"effective_to"`
}

type GetPriceAmountByID struct {
	ID string
}

type Service interface {
	Create(ctx context.Context, req CreateRequest) (*Response, error)
	List(ctx context.Context, req ListPriceAmountRequest) ([]Response, error)
	Get(ctx context.Context, req GetPriceAmountByID) (*Response, error)
}

type CreateRequest struct {
	PriceID            string         `json:"price_id"`
	MeterID            *string        `json:"meter_id"`
	Currency           string         `json:"currency"`
	UnitAmountCents    int64          `json:"unit_amount_cents"`
	MinimumAmountCents *int64         `json:"minimum_amount_cents"`
	MaximumAmountCents *int64         `json:"maximum_amount_cents"`
	EffectiveFrom      *time.Time     `json:"effective_from,omitempty"`
	EffectiveTo        *time.Time     `json:"effective_to,omitempty"`
	Metadata           map[string]any `json:"metadata"`
}

type Response struct {
	ID                 snowflake.ID  `json:"id"`
	OrganizationID     snowflake.ID  `json:"organization_id"`
	PriceID            snowflake.ID  `json:"price_id"`
	MeterID            *snowflake.ID `json:"meter_id,omitempty"`
	Currency           string        `json:"currency"`
	UnitAmountCents    int64         `json:"unit_amount_cents"`
	MinimumAmountCents *int64        `json:"minimum_amount_cents,omitempty"`
	MaximumAmountCents *int64        `json:"maximum_amount_cents,omitempty"`
	EffectiveFrom      time.Time     `json:"effective_from"`
	EffectiveTo        *time.Time    `json:"effective_to,omitempty"`
	RevokedAt          *time.Time    `json:"revoked_at"`
	RevokedReason      *string       `json:"revoked_reason"`
	Status             string        `json:"status"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

var (
	ErrUpcomingAlreadyExists = errors.New("upcoming_already_exists")
	ErrInvalidOrganization   = errors.New("invalid_organization")
	ErrInvalidPrice          = errors.New("invalid_price")
	ErrInvalidCurrency       = errors.New("invalid_currency")
	ErrInvalidUnitAmount     = errors.New("invalid_unit_amount")
	ErrInvalidMinAmount      = errors.New("invalid_minimum_amount")
	ErrInvalidMaxAmount      = errors.New("invalid_maximum_amount")
	ErrInvalidMeterID        = errors.New("invalid_meter_id")
	ErrInvalidID             = errors.New("invalid_id")
	ErrInvalidEffectiveFrom  = errors.New("invalid_effective_from")
	ErrInvalidEffectiveTo    = errors.New("invalid_effective_to")
	ErrEffectiveOverlap      = errors.New("effective_range_overlap")
	ErrEffectiveGap          = errors.New("effective_range_gap")
	ErrNotFound              = errors.New("not_found")
)
