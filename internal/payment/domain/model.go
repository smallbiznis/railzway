package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

type EventRecord struct {
	ID              snowflake.ID   `json:"id" gorm:"primaryKey"`
	OrgID           snowflake.ID   `json:"org_id" gorm:"not null;index"`
	Provider        string         `json:"provider" gorm:"type:text;not null"`
	ProviderEventID string         `json:"provider_event_id" gorm:"type:text;not null"`
	EventType       string         `json:"event_type" gorm:"type:text;not null"`
	CustomerID      snowflake.ID   `json:"customer_id" gorm:"not null;index"`
	Payload         datatypes.JSON `json:"payload" gorm:"type:jsonb;not null"`
	ReceivedAt      time.Time      `json:"received_at" gorm:"not null"`
	ProcessedAt     *time.Time     `json:"processed_at"`
}

func (EventRecord) TableName() string { return "payment_events" }

const (
	EventTypePaymentSucceeded = "payment_succeeded"
	EventTypePaymentFailed    = "payment_failed"
	EventTypeRefunded         = "refunded"
)

// PaymentEvent is the canonical payment event parsed by adapters.
type PaymentEvent struct {
	Provider            string
	ProviderEventID     string
	ProviderPaymentID   string
	ProviderPaymentType string
	Type                string
	OrgID               snowflake.ID
	CustomerID          snowflake.ID
	Amount              int64
	Currency            string
	OccurredAt          time.Time
	RawPayload          []byte
	InvoiceID           *snowflake.ID
}
