package domain

import (
	"context"
	"net/http"

	"github.com/bwmarrin/snowflake"
)

type PaymentAdapter interface {
	Verify(ctx context.Context, payload []byte, headers http.Header) error
	Parse(ctx context.Context, payload []byte) (*PaymentEvent, error)
}

type AdapterConfig struct {
	OrgID    snowflake.ID
	Provider string
	Config   map[string]any
}

type AdapterFactory interface {
	Provider() string
	NewAdapter(config AdapterConfig) (PaymentAdapter, error)
}
