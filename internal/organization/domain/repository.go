package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
)

type OrganizationListItem struct {
	ID        snowflake.ID
	Name      string
	Role      string
	CreatedAt time.Time
}

type Repository interface {
	WithTx(tx *gorm.DB) Repository
	CreateOrganization(ctx context.Context, org Organization) error
	AddMember(ctx context.Context, member OrganizationMember) error
	ListOrganizationsByUser(ctx context.Context, userID snowflake.ID) ([]OrganizationListItem, error)
	IsMember(ctx context.Context, orgID snowflake.ID, userID snowflake.ID) (bool, error)
	CreateInvites(ctx context.Context, invites []OrganizationInvite) error
	GetInvite(ctx context.Context, inviteID snowflake.ID) (*OrganizationInvite, error)
	UpdateInvite(ctx context.Context, invite OrganizationInvite) error
	UpsertBillingPreferences(ctx context.Context, prefs OrganizationBillingPreferences) error
}
