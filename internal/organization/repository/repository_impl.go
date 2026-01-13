package repository

import (
	"context"
	"errors"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/railzway/internal/organization/domain"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) domain.Repository {
	return &repository{db: db}
}

func (r *repository) WithTx(tx *gorm.DB) domain.Repository {
	return &repository{db: tx}
}

func (r *repository) CreateOrganization(ctx context.Context, org domain.Organization) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO organizations (id, name, slug, country_code, timezone_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		org.ID,
		org.Name,
		org.Slug,
		org.CountryCode,
		org.TimezoneName,
		org.CreatedAt,
	).Error
}

func (r *repository) AddMember(ctx context.Context, member domain.OrganizationMember) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO organization_members (id, org_id, user_id, role, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		member.ID,
		member.OrgID,
		member.UserID,
		member.Role,
		member.CreatedAt,
	).Error
}

func (r *repository) ListOrganizationsByUser(ctx context.Context, userID snowflake.ID) ([]domain.OrganizationListItem, error) {
	var items []domain.OrganizationListItem
	err := r.db.WithContext(ctx).Raw(
		`SELECT o.id, o.name, m.role, o.created_at
		 FROM organizations o
		 JOIN organization_members m ON m.org_id = o.id
		 WHERE m.user_id = ?
		 ORDER BY o.created_at ASC`,
		userID,
	).Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (r *repository) IsMember(ctx context.Context, orgID snowflake.ID, userID snowflake.ID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *repository) CreateInvites(ctx context.Context, invites []domain.OrganizationInvite) error {
	if len(invites) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&invites).Error
}

func (r *repository) UpsertBillingPreferences(ctx context.Context, prefs domain.OrganizationBillingPreferences) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO organization_billing_preferences (org_id, currency, timezone, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (org_id)
		 DO UPDATE SET currency = EXCLUDED.currency,
		               timezone = EXCLUDED.timezone,
		               updated_at = EXCLUDED.updated_at`,
		prefs.OrgID,
		prefs.Currency,
		prefs.Timezone,
		prefs.CreatedAt,
		prefs.UpdatedAt,
	).Error
}

func (r *repository) GetInvite(ctx context.Context, inviteID snowflake.ID) (*domain.OrganizationInvite, error) {
	var invite domain.OrganizationInvite
	err := r.db.WithContext(ctx).First(&invite, "id = ?", inviteID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Or return a specific error if preferred, but nil, nil is common for "not found" in some patterns. However, standard is usually error.
		}
		return nil, err
	}
	return &invite, nil
}

func (r *repository) UpdateInvite(ctx context.Context, invite domain.OrganizationInvite) error {
	return r.db.WithContext(ctx).Save(&invite).Error
}
