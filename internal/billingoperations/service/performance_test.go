package service

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/glebarez/sqlite"
	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestCalculatePerformance(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})

	// Setup schema
	db.Exec(`CREATE TABLE IF NOT EXISTS billing_operation_assignments (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id BIGINT NOT NULL,
		assigned_to TEXT NOT NULL,
		assigned_at TIMESTAMP NOT NULL,
		assignment_expires_at TIMESTAMP NOT NULL,
		status TEXT NOT NULL,
		released_at TIMESTAMP,
		breached_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS billing_operation_actions (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id BIGINT NOT NULL,
		action_type TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		metadata TEXT
	)`)

	node, _ := snowflake.NewNode(1)
	svc := &Service{
		db:    db,
		log:   zap.NewNop(),
		clock: clock.SystemClock{},
		genID: node,
	}

	orgID := node.Generate()
	userID := "user_test"
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	end := now

	ctx := orgcontext.WithOrgID(context.Background(), int64(orgID))

	t.Run("Score Calculation Logic", func(t *testing.T) {
		// Clean tables
		db.Exec("DELETE FROM billing_operation_assignments")
		db.Exec("DELETE FROM billing_operation_actions")

		// 1. Resolved Assignment (Fast)
		a1 := node.Generate()
		db.Exec("INSERT INTO billing_operation_assignments (id, org_id, entity_type, entity_id, assigned_to, assigned_at, assignment_expires_at, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			a1, orgID, "invoice", 101, userID, start.Add(1*time.Hour), start.Add(24*time.Hour), domain.AssignmentStatusReleased, now, now)
		// Action 30 mins later
		db.Exec("INSERT INTO billing_operation_actions (id, org_id, entity_type, entity_id, action_type, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			node.Generate(), orgID, "invoice", 101, domain.ActionTypeFollowUp, start.Add(90*time.Minute))
		// Release action with snapshot
		// Use struct to ensure JSONMap is stored compatibly
		type Action struct {
			ID         int64
			OrgID      int64
			EntityType string
			EntityID   int64
			ActionType string
			CreatedAt  time.Time
			Metadata   datatypes.JSONMap
		}
		db.Table("billing_operation_actions").Create(&Action{
			ID:         node.Generate().Int64(),
			OrgID:      orgID.Int64(),
			EntityType: "invoice",
			EntityID:   101,
			ActionType: domain.ActionTypeRelease,
			CreatedAt:  start.Add(2 * time.Hour),
			Metadata:   datatypes.JSONMap{"snapshot": map[string]any{"amount_due": 50000}},
		})

		// 2. Escalated Assignment
		a2 := node.Generate()
		db.Exec("INSERT INTO billing_operation_assignments (id, org_id, entity_type, entity_id, assigned_to, assigned_at, assignment_expires_at, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			a2, orgID, "invoice", 102, userID, start.Add(3*time.Hour), start.Add(24*time.Hour), domain.AssignmentStatusEscalated, now, now)

		// 3. Outstanding Assignment (Slow)
		a3 := node.Generate()
		db.Exec("INSERT INTO billing_operation_assignments (id, org_id, entity_type, entity_id, assigned_to, assigned_at, assignment_expires_at, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			a3, orgID, "invoice", 103, userID, start.Add(5*time.Hour), start.Add(24*time.Hour), domain.AssignmentStatusAssigned, now, now)

		snap, err := svc.CalculatePerformance(ctx, userID, start, end)
		assert.NoError(t, err)

		// Metrics Verification
		assert.Equal(t, 3, snap.Metrics.TotalAssigned)
		assert.Equal(t, 1, snap.Metrics.TotalResolved)
		assert.Equal(t, 1, snap.Metrics.TotalEscalated)
		// Exposure: 50000
		assert.Equal(t, int64(50000), snap.Metrics.ExposureHandled)

		// Completion Ratio: 1/3 = 0.33
		// Escalation Rate: 1/3 = 0.33

		// Scores Verification (V1 Equal Weight)
		// Responsiveness: 30 minutes = 0.5 hours. <= 1h -> 100
		assert.Equal(t, 100, snap.Scores.Responsiveness)

		// Completion: 0.33 * 100 = 33
		assert.Equal(t, 33, snap.Scores.Completion)

		// Risk: (1 - 0.33) * 100 = 66 (approx)
		assert.Equal(t, 66, snap.Scores.Risk) // 1 - 0.3333 = 0.6666 -> 66

		// Effectiveness: 50k > 10k -> 75
		assert.Equal(t, 75, snap.Scores.Effectiveness)

		// Total: (100 + 33 + 66 + 75) / 4 = 274 / 4 = 68
		assert.Equal(t, 68, snap.Scores.Total)

		// Metadata Verification
		assert.Equal(t, domain.PeriodTypeDaily, snap.PeriodType)
		assert.Equal(t, domain.ScoringVersionV1EqualWeight, snap.ScoringVersion)
	})
}

func TestAggregateDailyPerformance_Immutability(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})

	// Setup schema
	db.Exec(`CREATE TABLE IF NOT EXISTS finops_performance_snapshots (
		id BIGINT PRIMARY KEY,
		org_id BIGINT NOT NULL,
		user_id TEXT NOT NULL,
		period_type TEXT NOT NULL,
		period_start TIMESTAMP NOT NULL,
		period_end TIMESTAMP NOT NULL,
		scoring_version TEXT NOT NULL,
		metrics TEXT NOT NULL,
		scores TEXT NOT NULL,
		total_score INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`)
	// Setup assignments table for active user check
	db.Exec(`CREATE TABLE IF NOT EXISTS billing_operation_assignments (
		id BIGINT PRIMARY KEY,
		org_id BIGINT,
		entity_type TEXT,
		entity_id BIGINT,
		assigned_to TEXT,
		assigned_at TIMESTAMP,
		assignment_expires_at TIMESTAMP,
		status TEXT,
		breached_at TIMESTAMP,
		created_at TIMESTAMP,
		updated_at TIMESTAMP
	)`)

	node, _ := snowflake.NewNode(1)
	svc := &Service{
		db:         db,
		log:        zap.NewNop(),
		clock:      clock.SystemClock{},
		genID:      node,
		billingCfg: &config.BillingConfigHolder{},
	}

	// Mock clock/time - Aggregate uses "Yesterday" relative to Now
	// But in test we can insert data for "Yesterday"
	now := time.Now().UTC()
	yesterdayStart := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)

	orgID := node.Generate()
	userID := "user_immu"

	// Seed assignment to trigger aggregation
	db.Exec("INSERT INTO billing_operation_assignments (id, org_id, entity_type, entity_id, assigned_to, assigned_at, assignment_expires_at, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		node.Generate(), orgID, "invoice", node.Generate(), userID, yesterdayStart.Add(1*time.Hour), yesterdayStart.Add(24*time.Hour), domain.AssignmentStatusAssigned, now, now)

	// 1. First Run
	// Mock CalculatePerformance internal call?
	// Since we can't easily mock internal method in same package test without interface or refactor,
	// We rely on CalculatePerformance running logic again (which returns empty/zeros if no real data found)
	// That's fine, we care about the snapshot entry creation.

	// Create "Actions/Assignments" tables needed for CalculatePerformance to not error out
	db.Exec(`CREATE TABLE IF NOT EXISTS billing_operation_actions (id BIGINT, org_id BIGINT, entity_id BIGINT, action_type TEXT, created_at TIMESTAMP, metadata TEXT)`)

	err := svc.AggregateDailyPerformance(context.Background())
	assert.NoError(t, err)

	var count int64
	db.Table("finops_performance_snapshots").Count(&count)
	assert.Equal(t, int64(1), count)

	var firstSnap struct {
		ID        string
		CreatedAt time.Time
	}
	db.Table("finops_performance_snapshots").First(&firstSnap)

	// 2. Second Run (Recompute)
	// Should Delete and Insert new
	// Wait a bit or manually update created_at to verify change?
	// The new insert will have a NEW ID (snowflake generated)

	err = svc.AggregateDailyPerformance(context.Background())
	assert.NoError(t, err)

	db.Table("finops_performance_snapshots").Count(&count)
	assert.Equal(t, int64(1), count) // Still 1 record due to delete

	var secondSnap struct {
		ID        string
		CreatedAt time.Time
	}
	db.Table("finops_performance_snapshots").First(&secondSnap)

	assert.NotEqual(t, firstSnap.ID, secondSnap.ID, "Snapshot ID should change (Delete+Insert)")
}
