package seed

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/auth/password"
	organizationdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"gorm.io/gorm"
)

const (
	defaultOrgName       = "Main"
	defaultOrgSlug       = "main"
	defaultAdminEmail    = "admin@valora.cloud"
	defaultAdminPassword = "admin"
	defaultAdminDisplay  = "Valora Admin"
)

// EnsureMainOrg seeds the default organization for startup bootstrap.
func EnsureMainOrg(db *gorm.DB) error {
	if db == nil {
		return errors.New("seed database handle is required")
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := ensureMainOrgTx(ctx, tx, node)
		return err
	})
}

// EnsureMainOrgAndAdmin seeds the default organization and admin user for OSS mode.
func EnsureMainOrgAndAdmin(db *gorm.DB) error {
	if db == nil {
		return errors.New("seed database handle is required")
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org, err := ensureMainOrgTx(ctx, tx, node)
		if err != nil {
			return err
		}

		var user authdomain.User
		err = tx.WithContext(ctx).
			Where("provider = ? AND external_id = ?", "local", defaultAdminEmail).
			First(&user).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			hashed, err := password.Hash(defaultAdminPassword)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			user = authdomain.User{
				ID:                  node.Generate(),
				ExternalID:          defaultAdminEmail,
				Provider:            "local",
				DisplayName:         defaultAdminDisplay,
				Email:               strings.ToLower(defaultAdminEmail),
				PasswordHash:        &hashed,
				LastPasswordChanged: nil,
				IsDefault:           true,
				CreatedAt:           now,
				UpdatedAt:           now,
			}
			if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
				return err
			}
		}

		var member organizationdomain.OrganizationMember
		err = tx.WithContext(ctx).
			Where("org_id = ? AND user_id = ?", org.ID, user.ID).
			First(&member).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			now := time.Now().UTC()
			member = organizationdomain.OrganizationMember{
				ID:        node.Generate(),
				OrgID:     org.ID,
				UserID:    user.ID,
				Role:      organizationdomain.RoleOwner,
				CreatedAt: now,
			}
			if err := tx.WithContext(ctx).Create(&member).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func ensureMainOrgTx(ctx context.Context, tx *gorm.DB, node *snowflake.Node) (organizationdomain.Organization, error) {
	var org organizationdomain.Organization
	err := tx.WithContext(ctx).Where("slug = ?", defaultOrgSlug).First(&org).Error
	if err == nil {
		return org, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return org, err
	}
	now := time.Now().UTC()
	org = organizationdomain.Organization{
		ID:        node.Generate(),
		Name:      defaultOrgName,
		Slug:      defaultOrgSlug,
		IsDefault: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.WithContext(ctx).Create(&org).Error; err != nil {
		return org, err
	}
	return org, nil
}
