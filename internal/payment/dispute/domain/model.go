package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

const (
	EventTypeDisputeCreated         = "dispute.created"
	EventTypeDisputeFundsWithdrawn  = "dispute.funds_withdrawn"
	EventTypeDisputeFundsReinstated = "dispute.funds_reinstated"
	EventTypeDisputeClosed          = "dispute.closed"
)

const (
	DisputeStatusOpen       = "open"
	DisputeStatusWithdrawn  = "withdrawn"
	DisputeStatusReinstated = "reinstated"
	DisputeStatusClosed     = "closed"
)

// DisputeEvent is the canonical dispute event parsed by adapters.
type DisputeEvent struct {
	Provider          string
	ProviderEventID   string
	ProviderDisputeID string
	Type              string
	OrgID             snowflake.ID
	CustomerID        snowflake.ID
	Amount            int64
	Currency          string
	Reason            string
	OccurredAt        time.Time
	RawPayload        []byte
}

// DisputeRecord stores the normalized dispute lifecycle.
type DisputeRecord struct {
	ID                snowflake.ID `gorm:"primaryKey"`
	OrgID             snowflake.ID `gorm:"not null;index"`
	Provider          string       `gorm:"type:text;not null"`
	ProviderDisputeID string       `gorm:"type:text;not null"`
	ProviderEventID   string       `gorm:"type:text;not null"`
	CustomerID        snowflake.ID `gorm:"not null;index"`
	Amount            int64        `gorm:"not null"`
	Currency          string       `gorm:"type:text;not null"`
	Status            string       `gorm:"type:text;not null"`
	Reason            string       `gorm:"type:text"`
	ReceivedAt        time.Time    `gorm:"not null"`
	ProcessedAt       *time.Time
}

func (DisputeRecord) TableName() string { return "payment_disputes" }
