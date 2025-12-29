package domain

import (
	"context"
	"errors"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type ListAuditLogRequest struct {
	pagination.Pagination
	Action     string
	TargetType string
	TargetID   string
	ActorType  string
	StartAt    *time.Time
	EndAt      *time.Time
}

type ListAuditLogResponse struct {
	pagination.PageInfo
	AuditLogs []AuditLog `json:"audit_logs"`
}

type Service interface {
	AuditLog(ctx context.Context, orgID *snowflake.ID, actorType string, actorID *string, action string, targetType string, targetID *string, metadata map[string]any) error
	List(ctx context.Context, req ListAuditLogRequest) (ListAuditLogResponse, error)
}

var (
	ErrInvalidOrganization = errors.New("invalid_organization")
	ErrInvalidPageToken    = errors.New("invalid_page_token")
	ErrInvalidTimeRange    = errors.New("invalid_time_range")
	ErrInvalidAction       = errors.New("invalid_action")
)
