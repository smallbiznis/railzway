package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	auditcontext "github.com/smallbiznis/valora/internal/auditcontext"
	"github.com/smallbiznis/valora/internal/orgcontext"
	"github.com/smallbiznis/valora/pkg/db/pagination"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Params struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
	Repo  auditdomain.Repository
}

type Service struct {
	db    *gorm.DB
	log   *zap.Logger
	genID *snowflake.Node
	repo  auditdomain.Repository
}

func NewService(p Params) auditdomain.Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("audit.service"),
		genID: p.GenID,
		repo:  p.Repo,
	}
}

func (s *Service) AuditLog(ctx context.Context, orgID *snowflake.ID, actorType string, actorID *string, action string, targetType string, targetID *string, metadata map[string]any) error {
	action = strings.TrimSpace(action)
	if action == "" {
		return auditdomain.ErrInvalidAction
	}

	actorType = strings.TrimSpace(actorType)
	targetType = strings.TrimSpace(targetType)
	if targetType == "" {
		targetType = "unknown"
	}

	resolvedOrgID := s.resolveOrgID(ctx, orgID)

	resolvedActorType, resolvedActorID := s.resolveActor(ctx, actorType, actorID)
	ipAddress := auditcontext.IPAddressFromContext(ctx)
	userAgent := auditcontext.UserAgentFromContext(ctx)

	payload := map[string]any{}
	for key, value := range metadata {
		if key == "" {
			continue
		}
		payload[key] = value
	}

	if requestID := auditcontext.RequestIDFromContext(ctx); requestID != "" {
		payload["request_id"] = requestID
	}
	if subscriptionID := auditcontext.SubscriptionIDFromContext(ctx); subscriptionID != "" {
		payload["subscription_id"] = subscriptionID
	}
	if billingCycleID := auditcontext.BillingCycleIDFromContext(ctx); billingCycleID != "" {
		payload["billing_cycle_id"] = billingCycleID
	}

	entry := auditdomain.AuditLog{
		ID:         s.genID.Generate(),
		OrgID:      resolvedOrgID,
		ActorType:  resolvedActorType,
		ActorID:    resolvedActorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   normalizePointer(targetID),
		Metadata:   datatypes.JSONMap(payload),
		CreatedAt:  time.Now().UTC(),
	}
	if ipAddress != "" {
		entry.IPAddress = &ipAddress
	}
	if userAgent != "" {
		entry.UserAgent = &userAgent
	}

	if err := s.repo.Insert(ctx, s.db, &entry); err != nil {
		s.log.Warn("failed to write audit log", zap.String("action", action), zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context, req auditdomain.ListAuditLogRequest) (auditdomain.ListAuditLogResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return auditdomain.ListAuditLogResponse{}, err
	}

	if req.StartAt != nil && req.EndAt != nil && req.StartAt.After(*req.EndAt) {
		return auditdomain.ListAuditLogResponse{}, auditdomain.ErrInvalidTimeRange
	}

	var cursor *auditdomain.AuditCursor
	if strings.TrimSpace(req.PageToken) != "" {
		decoded, err := pagination.DecodeCursor(req.PageToken)
		if err != nil {
			return auditdomain.ListAuditLogResponse{}, auditdomain.ErrInvalidPageToken
		}
		createdAt, err := time.Parse(time.RFC3339, decoded.CreatedAt)
		if err != nil {
			return auditdomain.ListAuditLogResponse{}, auditdomain.ErrInvalidPageToken
		}
		id, err := snowflake.ParseString(strings.TrimSpace(decoded.ID))
		if err != nil || id == 0 {
			return auditdomain.ListAuditLogResponse{}, auditdomain.ErrInvalidPageToken
		}
		cursor = &auditdomain.AuditCursor{
			ID:        id,
			CreatedAt: createdAt,
		}
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 250 {
		pageSize = 250
	}

	items, err := s.repo.List(ctx, s.db, auditdomain.ListFilter{
		OrgID:      orgID,
		Action:     req.Action,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		ActorType:  req.ActorType,
		StartAt:    req.StartAt,
		EndAt:      req.EndAt,
		Cursor:     cursor,
		Limit:      int(pageSize),
	})
	if err != nil {
		return auditdomain.ListAuditLogResponse{}, err
	}

	pageInfo := pagination.BuildCursorPageInfo(items, int32(pageSize), func(item *auditdomain.AuditLog) string {
		token, err := pagination.EncodeCursor(pagination.Cursor{
			ID:        item.ID.String(),
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
		})
		if err != nil {
			return ""
		}
		return token
	})
	if pageInfo != nil && pageInfo.HasMore && len(items) > int(pageSize) {
		items = items[:pageSize]
	}

	logs := make([]auditdomain.AuditLog, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		logs = append(logs, *item)
	}

	resp := auditdomain.ListAuditLogResponse{AuditLogs: logs}
	if pageInfo != nil {
		resp.PageInfo = *pageInfo
	}
	return resp, nil
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, auditdomain.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}

func (s *Service) resolveOrgID(ctx context.Context, orgID *snowflake.ID) *snowflake.ID {
	if orgID != nil && *orgID != 0 {
		return orgID
	}
	resolved, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || resolved == 0 {
		return nil
	}
	parsed := snowflake.ID(resolved)
	return &parsed
}

func (s *Service) resolveActor(ctx context.Context, actorType string, actorID *string) (string, *string) {
	if actorType == "" {
		if ctxType, ctxID := auditcontext.ActorFromContext(ctx); ctxType != "" {
			actorType = ctxType
			if actorID == nil || strings.TrimSpace(*actorID) == "" {
				if ctxID != "" {
					actorID = &ctxID
				}
			}
		}
	}
	if actorType == "" {
		actorType = string(auditdomain.ActorTypeSystem)
	}

	return actorType, normalizePointer(actorID)
}

func normalizePointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
