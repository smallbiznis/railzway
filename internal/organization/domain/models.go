// Package domain contains persistence models for the org service.
package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

// Organization represents a tenant.
type Organization struct {
	ID           snowflake.ID      `gorm:"primaryKey" json:"id"`
	Name         string            `gorm:"type:text;not null" json:"name"`
	Slug         string            `gorm:"type:text;not null;uniqueIndex:ux_organizations_slug" json:"slug"`
	SupportEmail string            `gorm:"type:text;column:support_email" json:"support_email"`
	IsDefault    bool              `gorm:"column:is_default" json:"is_default"`
	CountryCode  string            `gorm:"column:country_code"`
	TimezoneName string            `gorm:"column:timezone_name"`
	Metadata     datatypes.JSONMap `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt    time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName sets the database table name.
func (Organization) TableName() string { return "organizations" }

// OrganizationUser represents membership of a user in an organization.
type OrganizationMember struct {
	ID        snowflake.ID `gorm:"primaryKey" json:"id"`
	OrgID     snowflake.ID `gorm:"not null;index;uniqueIndex:ux_org_user,priority:1" json:"org_id"`
	UserID    snowflake.ID `gorm:"not null;index;uniqueIndex:ux_org_user,priority:2" json:"user_id"`
	Role      string       `gorm:"type:text;not null" json:"role"`
	CreatedAt time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName sets the database table name.
func (OrganizationMember) TableName() string { return "organization_members" }

// OrganizationInvite tracks a pending invite to an organization.
type OrganizationInvite struct {
	ID        snowflake.ID `gorm:"primaryKey" json:"id"`
	OrgID     snowflake.ID `gorm:"not null;index" json:"org_id"`
	Email     string       `gorm:"type:text;not null" json:"email"`
	Role      string       `gorm:"type:text;not null" json:"role"`
	Status    string       `gorm:"type:text;not null" json:"status"`
	InvitedBy snowflake.ID `gorm:"column:invited_by;not null;index" json:"invited_by"`
	CreatedAt time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName sets the database table name.
func (OrganizationInvite) TableName() string { return "organization_invites" }

// OrganizationBillingPreferences stores billing defaults for an organization.
type OrganizationBillingPreferences struct {
	OrgID     snowflake.ID `gorm:"primaryKey" json:"org_id"`
	Currency  string       `gorm:"type:text;not null" json:"currency"`
	Timezone  string       `gorm:"type:text;not null" json:"timezone"`
	CreatedAt time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName sets the database table name.
func (OrganizationBillingPreferences) TableName() string { return "organization_billing_preferences" }
