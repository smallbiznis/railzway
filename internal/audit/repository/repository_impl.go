package repository

import (
	"context"
	"strings"

	"github.com/smallbiznis/valora/internal/audit/domain"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() domain.Repository {
	return &repo{}
}

func (r *repo) Insert(ctx context.Context, db *gorm.DB, entry *domain.AuditLog) error {
	if entry == nil {
		return nil
	}
	return db.WithContext(ctx).Exec(
		`INSERT INTO audit_logs (
			id, org_id, actor_type, actor_id, action, target_type, target_id,
			metadata, ip_address, user_agent, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.OrgID,
		entry.ActorType,
		entry.ActorID,
		entry.Action,
		entry.TargetType,
		entry.TargetID,
		entry.Metadata,
		entry.IPAddress,
		entry.UserAgent,
		entry.CreatedAt,
	).Error
}

func (r *repo) List(ctx context.Context, db *gorm.DB, filter domain.ListFilter) ([]*domain.AuditLog, error) {
	var logs []*domain.AuditLog
	stmt := db.WithContext(ctx).Model(&domain.AuditLog{}).
		Where("org_id = ?", filter.OrgID)

	if action := strings.TrimSpace(filter.Action); action != "" {
		stmt = stmt.Where("action = ?", action)
	}
	if targetType := strings.TrimSpace(filter.TargetType); targetType != "" {
		stmt = stmt.Where("target_type = ?", targetType)
	}
	if targetID := strings.TrimSpace(filter.TargetID); targetID != "" {
		stmt = stmt.Where("target_id = ?", targetID)
	}
	if actorType := strings.TrimSpace(filter.ActorType); actorType != "" {
		stmt = stmt.Where("actor_type = ?", actorType)
	}
	if filter.StartAt != nil {
		stmt = stmt.Where("created_at >= ?", filter.StartAt.UTC())
	}
	if filter.EndAt != nil {
		stmt = stmt.Where("created_at <= ?", filter.EndAt.UTC())
	}
	if filter.Cursor != nil {
		stmt = stmt.Where("(created_at < ?) OR (created_at = ? AND id < ?)",
			filter.Cursor.CreatedAt,
			filter.Cursor.CreatedAt,
			filter.Cursor.ID,
		)
	}

	stmt = stmt.Order("created_at desc, id desc")
	if filter.Limit > 0 {
		stmt = stmt.Limit(filter.Limit + 1)
	}

	if err := stmt.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
