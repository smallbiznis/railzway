package domain

import (
	"context"
	"errors"
	"time"

	"github.com/bwmarrin/snowflake"
)

const (
	RoleOwner     = "OWNER"
	RoleAdmin     = "ADMIN"
	RoleFinOps    = "FINOPS"    // View Invoices, Reports, Payments
	RoleDeveloper = "DEVELOPER" // API Keys, Webhooks
	RoleMember    = "MEMBER"    // Read-only / Limited
)

type Service interface {
	Create(ctx context.Context, userID snowflake.ID, req CreateOrganizationRequest) (*OrganizationResponse, error)
	GetByID(ctx context.Context, id string) (*OrganizationResponse, error)
	ListOrganizationsByUser(ctx context.Context, userID snowflake.ID) ([]OrganizationListResponseItem, error)
	InviteMembers(ctx context.Context, userID snowflake.ID, orgID string, invites []InviteRequest) error
	AcceptInvite(ctx context.Context, userID snowflake.ID, inviteID string) error
	GetInvite(ctx context.Context, inviteID string) (*PublicInviteInfo, error)
	SetBillingPreferences(ctx context.Context, userID snowflake.ID, orgID string, req BillingPreferencesRequest) error
}

type PublicInviteInfo struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	OrgName   string `json:"org_name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	InvitedBy string `json:"invited_by"`
}

type CreateOrganizationRequest struct {
	Name         string
	CountryCode  string
	TimezoneName string
}

type InviteRequest struct {
	Email string
	Role  string
}

type BillingPreferencesRequest struct {
	Currency string
	Timezone string
}

type OrganizationResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	CountryCode  string `json:"country_code"`
	TimezoneName string `json:"timezone_name"`
}

type OrganizationListResponseItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrInvalidName         = errors.New("invalid_name")
	ErrInvalidCountry      = errors.New("invalid_country")
	ErrInvalidTimezone     = errors.New("invalid_timezone")
	ErrInvalidCurrency     = errors.New("invalid_currency")
	ErrInvalidUser         = errors.New("invalid_user")
	ErrInvalidOrganization = errors.New("invalid_organization")
	ErrInvalidEmail        = errors.New("invalid_email")
	ErrInvalidRole         = errors.New("invalid_role")
	ErrForbidden           = errors.New("forbidden")
)
