package domain

import (
	"context"
	"errors"
	"time"
)

type Service interface {
	List(ctx context.Context) ([]Response, error)
	Create(ctx context.Context, req CreateRequest) (*SecretResponse, error)
	Rotate(ctx context.Context, keyID string) (*SecretResponse, error)
	Revoke(ctx context.Context, keyID string) error
}

type CreateRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type Response struct {
	KeyID            string     `json:"key_id"`
	Name             string     `json:"name"`
	Scopes           []string   `json:"scopes"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
	LastUsedAt       *time.Time `json:"last_used_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	RotatedFromKeyID *string    `json:"rotated_from_key_id"`
}

type SecretResponse struct {
	KeyID  string `json:"key_id"`
	APIKey string `json:"api_key"`
}

var (
	ErrInvalidOrganization = errors.New("invalid_organization")
	ErrInvalidName         = errors.New("invalid_name")
	ErrInvalidKeyID        = errors.New("invalid_key_id")
	ErrNotFound            = errors.New("not_found")
)
