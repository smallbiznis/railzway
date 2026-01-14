package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	auditcontext "github.com/smallbiznis/railzway/internal/auditcontext"
	billingoperationsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

// RecordFollowUp records that a follow-up email was opened for an assignment
// This is a simple tracking method - the actual email is composed in the user's email client
func (s *Service) RecordFollowUp(ctx context.Context, req billingoperationsdomain.RecordFollowUpRequest) error {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return billingoperationsdomain.ErrInvalidOrganization
	}

	_, userID := auditcontext.ActorFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("invalid_user")
	}

	// Parse and validate assignment ID
	assignmentID, err := snowflake.ParseString(strings.TrimSpace(req.AssignmentID))
	if err != nil {
		return fmt.Errorf("invalid_assignment_id")
	}

	// Load assignment
	var assignment struct {
		ID         snowflake.ID
		OrgID      snowflake.ID
		EntityType string
		EntityID   snowflake.ID
		AssignedTo string
		Status     string
		Metadata   datatypes.JSONMap
	}

	if err := s.db.WithContext(ctx).
		Table("billing_operation_assignments").
		Where("id = ? AND org_id = ?", assignmentID, orgID).
		First(&assignment).Error; err != nil {
		return fmt.Errorf("assignment not found: %w", err)
	}

	// Verify ownership
	if assignment.AssignedTo != userID {
		return fmt.Errorf("assignment_not_owned")
	}

	// Update assignment metadata
	now := s.clock.Now().UTC()
	metadata := assignment.Metadata
	if metadata == nil {
		metadata = datatypes.JSONMap{}
	}

	// Increment follow-up count
	followUpCount := 0
	if count, ok := metadata["follow_up_count"].(float64); ok {
		followUpCount = int(count)
	}
	followUpCount++

	metadata["follow_up_count"] = followUpCount
	metadata["last_follow_up_at"] = now.Format(time.RFC3339)
	if req.EmailProvider != "" {
		metadata["last_email_provider"] = req.EmailProvider
	}

	// Update assignment
	if err := s.db.WithContext(ctx).
		Table("billing_operation_assignments").
		Where("id = ?", assignmentID).
		Updates(map[string]interface{}{
			"metadata":       metadata,
			"last_action_at": now,
		}).Error; err != nil {
		s.log.Error("failed to update assignment metadata", zap.Error(err))
		return err
	}

	// Create audit log entry
	if s.auditSvc != nil {
		targetID := assignment.EntityID.String()
		_ = s.auditSvc.AuditLog(ctx, &orgID, "", nil,
			"billing_operations.follow_up_opened",
			"billing_operation_assignment",
			&targetID,
			map[string]any{
				"assignment_id":   req.AssignmentID,
				"entity_type":     assignment.EntityType,
				"entity_id":       assignment.EntityID.String(),
				"email_provider":  req.EmailProvider,
				"follow_up_count": followUpCount,
			},
		)
	}

	s.log.Info("follow-up email opened",
		zap.String("assignment_id", req.AssignmentID),
		zap.String("user_id", userID),
		zap.String("email_provider", req.EmailProvider),
		zap.Int("follow_up_count", followUpCount),
	)

	return nil
}
