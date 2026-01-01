package domain

import (
	"context"
	"errors"
	"net/http"
)

type Service interface {
	IngestWebhook(ctx context.Context, provider string, payload []byte, headers http.Header) error
}

var (
	ErrInvalidProvider       = errors.New("invalid_provider")
	ErrProviderNotFound      = errors.New("provider_not_found")
	ErrInvalidSignature      = errors.New("invalid_signature")
	ErrInvalidPayload        = errors.New("invalid_payload")
	ErrInvalidEvent          = errors.New("invalid_event")
	ErrEventIgnored          = errors.New("event_ignored")
	ErrInvalidCustomer       = errors.New("invalid_customer")
	ErrInvalidAmount         = errors.New("invalid_amount")
	ErrInvalidCurrency       = errors.New("invalid_currency")
	ErrInvalidConfig         = errors.New("invalid_config")
	ErrEventAlreadyProcessed = errors.New("event_already_processed")
)
