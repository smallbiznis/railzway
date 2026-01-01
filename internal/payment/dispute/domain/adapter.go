package domain

import "context"

// DisputeAdapter parses provider-specific dispute events.
type DisputeAdapter interface {
	ParseDispute(ctx context.Context, payload []byte) (*DisputeEvent, error)
}
