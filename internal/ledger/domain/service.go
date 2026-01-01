package domain

import (
	"context"
	"errors"
	"time"

	"github.com/bwmarrin/snowflake"
)

// LedgerService defines the ledger entry writer.
type LedgerService interface {
	CreateEntry(
		ctx context.Context,
		orgID snowflake.ID,
		sourceType string,
		sourceID snowflake.ID,
		currency string,
		occurredAt time.Time,
		lines []LedgerEntryLine,
	) error
}

// Service is the package alias for LedgerService.
type Service = LedgerService

var (
	ErrInvalidOrganization  = errors.New("invalid_organization")
	ErrInvalidSourceType    = errors.New("invalid_source_type")
	ErrInvalidSourceID      = errors.New("invalid_source_id")
	ErrInvalidCurrency      = errors.New("invalid_currency")
	ErrInvalidOccurredAt    = errors.New("invalid_occurred_at")
	ErrInvalidEntryLines    = errors.New("invalid_entry_lines")
	ErrInvalidLineAmount    = errors.New("invalid_line_amount")
	ErrInvalidLineDirection = errors.New("invalid_line_direction")
	ErrInvalidAccount       = errors.New("invalid_account")
	ErrUnbalancedEntry      = errors.New("unbalanced_entry")
)
