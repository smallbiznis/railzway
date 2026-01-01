package events

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Event describes a billing event to store in the outbox.
type Event struct {
	OrgID     snowflake.ID
	Type      string
	Payload   map[string]any
	DedupeKey string
}

// Outbox inserts billing events into the billing_events table.
type Outbox struct {
	db    *gorm.DB
	genID *snowflake.Node
}

func NewOutbox(db *gorm.DB, genID *snowflake.Node) *Outbox {
	return &Outbox{db: db, genID: genID}
}

// Publish stores an event using the default database connection.
func (o *Outbox) Publish(ctx context.Context, event Event) error {
	return o.publish(ctx, o.db, event)
}

// PublishTx stores an event using an existing transaction.
func (o *Outbox) PublishTx(ctx context.Context, tx *gorm.DB, event Event) error {
	if tx == nil {
		return errors.New("missing_transaction")
	}
	return o.publish(ctx, tx, event)
}

func (o *Outbox) publish(ctx context.Context, db *gorm.DB, event Event) error {
	if o == nil || db == nil || o.genID == nil {
		return errors.New("outbox_unavailable")
	}
	if event.OrgID == 0 {
		return errors.New("invalid_org_id")
	}
	name := strings.TrimSpace(event.Type)
	if name == "" {
		return errors.New("missing_event_type")
	}

	payload := datatypes.JSONMap{}
	for key, value := range event.Payload {
		if strings.TrimSpace(key) == "" {
			continue
		}
		payload[key] = value
	}

	dedupe := strings.TrimSpace(event.DedupeKey)
	var dedupeValue any
	if dedupe != "" {
		dedupeValue = dedupe
	}

	now := time.Now().UTC()
	return db.WithContext(ctx).Exec(
		`INSERT INTO billing_events (id, org_id, event_type, payload, dedupe_key, published, created_at)
		 VALUES (?, ?, ?, ?, ?, false, ?)
		 ON CONFLICT (org_id, dedupe_key) DO NOTHING`,
		o.genID.Generate(),
		event.OrgID,
		name,
		payload,
		dedupeValue,
		now,
	).Error
}
