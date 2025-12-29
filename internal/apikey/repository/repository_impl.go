package repository

import (
	"context"

	"github.com/bwmarrin/snowflake"
	apikeydomain "github.com/smallbiznis/valora/internal/apikey/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() apikeydomain.Repository {
	return &repo{}
}

func (r *repo) Insert(ctx context.Context, db *gorm.DB, key *apikeydomain.APIKey) error {
	return db.WithContext(ctx).Exec(
		`INSERT INTO api_keys (id, org_id, key_id, name, scope, key_hash, is_active, created_at, updated_at, last_used_at, expires_at, rotated_from_key_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID,
		key.OrgID,
		key.KeyID,
		key.Name,
		key.Scope,
		key.KeyHash,
		key.IsActive,
		key.CreatedAt,
		key.UpdatedAt,
		key.LastUsedAt,
		key.ExpiresAt,
		key.RotatedFromKeyID,
	).Error
}

func (r *repo) Update(ctx context.Context, db *gorm.DB, key *apikeydomain.APIKey) error {
	return db.WithContext(ctx).Exec(
		`UPDATE api_keys
		 SET name = ?, scope = ?, key_hash = ?, is_active = ?, updated_at = ?, last_used_at = ?, expires_at = ?, rotated_from_key_id = ?
		 WHERE org_id = ? AND key_id = ?`,
		key.Name,
		key.Scope,
		key.KeyHash,
		key.IsActive,
		key.UpdatedAt,
		key.LastUsedAt,
		key.ExpiresAt,
		key.RotatedFromKeyID,
		key.OrgID,
		key.KeyID,
	).Error
}

func (r *repo) FindByKeyID(ctx context.Context, db *gorm.DB, orgID snowflake.ID, keyID string) (*apikeydomain.APIKey, error) {
	var key apikeydomain.APIKey
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, key_id, name, scope, key_hash, is_active, created_at, updated_at, last_used_at, expires_at, rotated_from_key_id
		 FROM api_keys WHERE org_id = ? AND key_id = ?`,
		orgID,
		keyID,
	).Scan(&key).Error
	if err != nil {
		return nil, err
	}
	if key.ID == 0 {
		return nil, nil
	}
	return &key, nil
}

func (r *repo) List(ctx context.Context, db *gorm.DB, orgID snowflake.ID) ([]apikeydomain.APIKey, error) {
	var keys []apikeydomain.APIKey
	err := db.WithContext(ctx).Raw(
		`SELECT id, org_id, key_id, name, scope, key_hash, is_active, created_at, updated_at, last_used_at, expires_at, rotated_from_key_id
		 FROM api_keys WHERE org_id = ? ORDER BY created_at DESC`,
		orgID,
	).Scan(&keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}
