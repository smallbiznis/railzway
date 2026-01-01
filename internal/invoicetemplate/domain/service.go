package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
)

type ListRequest struct {
	Name      string `form:"name"`
	IsDefault *bool  `form:"is_default"`
}

type CreateRequest struct {
	Name      string         `json:"name"`
	IsDefault bool           `json:"is_default"`
	Locale    string         `json:"locale"`
	Currency  string         `json:"currency"`
	Header    map[string]any `json:"header"`
	Footer    map[string]any `json:"footer"`
	Style     map[string]any `json:"style"`
}

type UpdateRequest struct {
	ID       string         `json:"id"`
	Name     *string        `json:"name"`
	Locale   *string        `json:"locale"`
	Currency *string        `json:"currency"`
	Header   map[string]any `json:"header"`
	Footer   map[string]any `json:"footer"`
	Style    map[string]any `json:"style"`
}

type Response struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"organization_id"`
	Name      string         `json:"name"`
	IsDefault bool           `json:"is_default"`
	Locale    string         `json:"locale"`
	Currency  string         `json:"currency"`
	Header    map[string]any `json:"header"`
	Footer    map[string]any `json:"footer"`
	Style     map[string]any `json:"style"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Service interface {
	Create(ctx context.Context, req CreateRequest) (*Response, error)
	List(ctx context.Context, req ListRequest) ([]Response, error)
	GetByID(ctx context.Context, id string) (*Response, error)
	Update(ctx context.Context, req UpdateRequest) (*Response, error)
	SetDefault(ctx context.Context, id string) (*Response, error)
}

func ParseID(raw string) (snowflake.ID, error) {
	return snowflake.ParseString(strings.TrimSpace(raw))
}

var (
	ErrInvalidOrganization = errors.New("invalid_organization")
	ErrInvalidID           = errors.New("invalid_id")
	ErrInvalidName         = errors.New("invalid_name")
	ErrInvalidCurrency     = errors.New("invalid_currency")
	ErrInvalidLocale       = errors.New("invalid_locale")
	ErrNotFound            = errors.New("not_found")
)
