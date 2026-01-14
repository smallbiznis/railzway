package service

import (
	"context"
	"testing"

	"github.com/bwmarrin/snowflake"
	"github.com/glebarez/sqlite"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	auditcontext "github.com/smallbiznis/railzway/internal/auditcontext"
	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Mock Audit Service
type mockAuditSvc struct {
	mock.Mock
}

func (m *mockAuditSvc) AuditLog(ctx context.Context, orgID *snowflake.ID, actorType string, actorID *string, action string, targetType string, targetID *string, metadata map[string]any) error {
	args := m.Called(ctx, orgID, actorType, actorID, action, targetType, targetID, metadata)
	return args.Error(0)
}

func (m *mockAuditSvc) List(ctx context.Context, req auditdomain.ListAuditLogRequest) (auditdomain.ListAuditLogResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(auditdomain.ListAuditLogResponse), args.Error(1)
}

func TestAssignmentLifecycle(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})

	// Migrate internal structs - need to specify table names explicitly for SQLite
	// Create tables manually to match production schema
	db.Exec(`CREATE TABLE IF NOT EXISTS billing_operation_assignments (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id BIGINT NOT NULL,
		assigned_to TEXT NOT NULL,
		assigned_at TIMESTAMP NOT NULL,
		assignment_expires_at TIMESTAMP NOT NULL,
		status TEXT NOT NULL DEFAULT 'assigned',
		released_at TIMESTAMP,
		released_by TEXT,
		release_reason TEXT,
		last_action_at TIMESTAMP,

		snapshot_metadata TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS billing_operation_actions (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id BIGINT NOT NULL,
		action_type TEXT NOT NULL,
		action_bucket TIMESTAMP NOT NULL,
		idempotency_key TEXT,
		metadata TEXT,
		actor_type TEXT,
		actor_id TEXT,
		created_at TIMESTAMP NOT NULL
	)`)

	// SQLite requires explicit UNIQUE index for ON CONFLICT to work
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ux_billing_assignments_entity ON billing_operation_assignments(org_id, entity_type, entity_id)")

	node, _ := snowflake.NewNode(1)
	logger := zap.NewNop()
	clk := clock.SystemClock{}
	mockAudit := new(mockAuditSvc)

	svc := NewService(Params{
		DB:       db,
		Log:      logger,
		Clock:    clk,
		GenID:    node,
		AuditSvc: mockAudit,
		Cfg:      config.Config{},
	})

	orgID := node.Generate()
	entityID := node.Generate()
	entityType := domain.EntityTypeInvoice
	// Mock Org Context
	ctx := orgcontext.WithOrgID(context.Background(), int64(orgID))
	// Mock Actor Context for implicit actor
	ctx = auditcontext.WithActor(ctx, "user", "user_123")

	t.Run("Claim Assignment - Success", func(t *testing.T) {
		// Mock Audit Log expectation
		mockAudit.On("AuditLog", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "billing_operations.assignment.claimed", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		resp, err := svc.ClaimAssignment(ctx, domain.ClaimAssignmentRequest{
			EntityType:           entityType,
			EntityID:             entityID.String(),
			AssignedTo:           "agent_007",
			AssignmentTTLMinutes: 60,
		})
		assert.NoError(t, err)
		assert.Equal(t, "agent_007", resp.Assignment.AssignedTo)
		assert.Equal(t, domain.AssignmentStatusAssigned, resp.Status)
	})

	t.Run("Claim Assignment - Already Assigned Failure", func(t *testing.T) {
		// Try to claim again with different agent
		_, err := svc.ClaimAssignment(ctx, domain.ClaimAssignmentRequest{
			EntityType:           entityType,
			EntityID:             entityID.String(),
			AssignedTo:           "agent_008",
			AssignmentTTLMinutes: 60,
		})
		assert.Error(t, err)
		assert.Equal(t, domain.ErrAssignmentConflict, err)
	})

	t.Run("Release Assignment - Success", func(t *testing.T) {
		// Mock Audit Log expectation
		mockAudit.On("AuditLog", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "billing_operations.assignment.released", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.ReleaseAssignment(ctx, domain.ReleaseAssignmentRequest{
			EntityType: entityType,
			EntityID:   entityID.String(),
			Reason:     "Resolved",
			ReleasedBy: "agent_007",
		})
		assert.NoError(t, err)

		// Verify DB State
		var assignment domain.BillingAssignmentRecord
		err = db.Where("org_id = ? AND entity_type = ? AND entity_id = ?", orgID, entityType, entityID).First(&assignment).Error
		assert.NoError(t, err)
		assert.Equal(t, domain.AssignmentStatusReleased, assignment.Status)
		assert.True(t, assignment.ReleasedAt.Valid)
		assert.Equal(t, "agent_007", assignment.ReleasedBy.String)
		assert.Equal(t, "Resolved", assignment.ReleaseReason.String)
	})

	t.Run("Claim Assignment - Reclaim Released Entity", func(t *testing.T) {
		// Mock Audit Log expectation
		mockAudit.On("AuditLog", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "billing_operations.assignment.claimed", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		resp, err := svc.ClaimAssignment(ctx, domain.ClaimAssignmentRequest{
			EntityType:           entityType,
			EntityID:             entityID.String(),
			AssignedTo:           "agent_008",
			AssignmentTTLMinutes: 60,
		})
		assert.NoError(t, err)
		assert.Equal(t, "agent_008", resp.Assignment.AssignedTo)
		assert.Equal(t, domain.AssignmentStatusAssigned, resp.Status)
	})

	t.Run("Release Assignment - Using Context Actor", func(t *testing.T) {
		// Mock Audit Log expectation
		mockAudit.On("AuditLog", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "billing_operations.assignment.released", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.ReleaseAssignment(ctx, domain.ReleaseAssignmentRequest{
			EntityType: entityType,
			EntityID:   entityID.String(),
			Reason:     "Escalated",
			// ReleasedBy is empty, should grab from context "user_123"
		})
		assert.NoError(t, err)

		// Verify DB State
		var assignment domain.BillingAssignmentRecord
		err = db.Where("org_id = ? AND entity_type = ? AND entity_id = ?", orgID, entityType, entityID).First(&assignment).Error
		assert.NoError(t, err)
		assert.Equal(t, domain.AssignmentStatusReleased, assignment.Status)
		assert.True(t, assignment.ReleasedAt.Valid)
		assert.Equal(t, "user_123", assignment.ReleasedBy.String)
		if assignment.ReleaseReason.String != "Escalated" {
			t.Logf("ReleaseReason mismatch. assignment: %+v", assignment)
		}
		assert.Equal(t, "Escalated", assignment.ReleaseReason.String)
	})
}
