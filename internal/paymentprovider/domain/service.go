package domain

import (
	"context"
	"errors"
)

type Service interface {
	ListCatalog(ctx context.Context) ([]CatalogProviderResponse, error)
	ListConfigs(ctx context.Context) ([]ConfigSummary, error)
	UpsertConfig(ctx context.Context, req UpsertRequest) (*ConfigSummary, error)
	SetActive(ctx context.Context, provider string, isActive bool) (*ConfigSummary, error)
}

type CatalogProviderResponse struct {
	Provider        string  `json:"provider"`
	DisplayName     string  `json:"display_name"`
	Description     *string `json:"description,omitempty"`
	SupportsWebhook bool    `json:"supports_webhook"`
	SupportsRefund  bool    `json:"supports_refund"`
}

type ConfigSummary struct {
	Provider   string `json:"provider"`
	IsActive   bool   `json:"is_active"`
	Configured bool   `json:"configured"`
}

type UpsertRequest struct {
	Provider string         `json:"provider"`
	Config   map[string]any `json:"config"`
}

var (
	ErrInvalidOrganization  = errors.New("invalid_organization")
	ErrInvalidProvider      = errors.New("invalid_provider")
	ErrInvalidConfig        = errors.New("invalid_config")
	ErrNotFound             = errors.New("not_found")
	ErrEncryptionKeyMissing = errors.New("encryption_key_missing")
)
