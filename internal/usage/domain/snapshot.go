package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

// SnapshotCandidate is a usage event eligible for async snapshot enrichment.
type SnapshotCandidate struct {
	ID         snowflake.ID
	OrgID      snowflake.ID
	CustomerID snowflake.ID
	MeterCode  string
	RecordedAt time.Time
}

// SnapshotUpdate contains resolved snapshot fields for a usage event.
type SnapshotUpdate struct {
	ID                 snowflake.ID
	SubscriptionID     snowflake.ID
	SubscriptionItemID *snowflake.ID
	MeterID            *snowflake.ID
	Status             string
	SnapshotAt         time.Time
}
