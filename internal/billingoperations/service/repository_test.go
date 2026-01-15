package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/glebarez/sqlite"
	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/billingoperations/repository"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestFinOpsSnapshotRepository(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})

	// Setup schema (matching 0033 migration simplified for internal struct)
	// Note: We use the struct tag mapping from finopsSnapshotRow
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

	repo := repository.NewFinOpsSnapshotRepository(db)
	node, _ := snowflake.NewNode(1)
	orgID := node.Generate()
	userID := "user_repo_test"
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	// Seed Data
	metricsJSON := `{"total_assigned": 10, "total_resolved": 8, "total_escalated": 1, "avg_response_ms": 1800000, "exposure_handled": 1000}`
	scoresJSON := `{"total": 100}`

	db.Exec(`INSERT INTO finops_performance_snapshots 
		(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.Generate().Int64(), orgID.Int64(), userID, domain.PeriodTypeDaily, start, end,
		domain.ScoringVersionV1EqualWeight, string(metricsJSON), string(scoresJSON), 100, now, now)

	// User 2 in same Org
	user2 := "user_repo_test_2"
	db.Exec(`INSERT INTO finops_performance_snapshots 
		(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.Generate().Int64(), orgID.Int64(), user2, domain.PeriodTypeDaily, start, end,
		domain.ScoringVersionV1EqualWeight, string(metricsJSON), string(scoresJSON), 90, now, now)

	// Different Org
	otherOrgID := node.Generate()
	db.Exec(`INSERT INTO finops_performance_snapshots 
		(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.Generate().Int64(), otherOrgID.Int64(), userID, domain.PeriodTypeDaily, start, end,
		domain.ScoringVersionV1EqualWeight, string(metricsJSON), string(scoresJSON), 100, now, now)

	ctx := orgcontext.WithOrgID(context.Background(), orgID.Int64())

	t.Run("FindByUser", func(t *testing.T) {
		snaps, err := repo.FindByUser(ctx, orgID, userID, domain.PeriodTypeDaily, start, start.Add(48*time.Hour))
		assert.NoError(t, err)
		assert.Len(t, snaps, 1)
		assert.Equal(t, userID, snaps[0].UserID)
		assert.Equal(t, domain.ScoringVersionV1EqualWeight, snaps[0].ScoringVersion)
		assert.Equal(t, 10, snaps[0].Metrics.TotalAssigned)
	})

	t.Run("FindByUser_InvalidOrg", func(t *testing.T) {
		otherCtx := orgcontext.WithOrgID(context.Background(), otherOrgID.Int64())
		// Trying to access orgID data with otherCtx (which has otherOrgID) but we pass orgID as arg?
		// Repo FindByUser takes orgID as arg.
		// Implementation checks: if ctxOrgID != orgID -> error

		snaps, err := repo.FindByUser(otherCtx, orgID, userID, domain.PeriodTypeDaily, start, end)
		assert.ErrorIs(t, err, domain.ErrInvalidOrganization)
		assert.Nil(t, snaps)
	})

	t.Run("FindByOrg", func(t *testing.T) {
		snaps, err := repo.FindByOrg(ctx, orgID, domain.PeriodTypeDaily, start, start.Add(48*time.Hour))
		assert.NoError(t, err)
		assert.Len(t, snaps, 2) // user_repo_test and user_repo_test_2

		// Order check: user_repo_test < user_repo_test_2
		assert.Equal(t, userID, snaps[0].UserID)
		assert.Equal(t, user2, snaps[1].UserID)
	})

	t.Run("MapRowsHelper", func(t *testing.T) {
		// Verify mapping handles JSON correctly via struct
	})

	t.Run("FindByUserWithLimit", func(t *testing.T) {
		// Mock data has 2 users, user_repo_test has 1 snapshot in Jan 1
		// Let's add more snapshots to test limit/ordering
		node2, _ := snowflake.NewNode(2)
		// Insert 2 more snapshots for user_repo_test on Jan 2 and Jan 3
		now2 := start.Add(24 * time.Hour)
		now3 := start.Add(48 * time.Hour)

		db.Exec(`INSERT INTO finops_performance_snapshots 
			(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			node2.Generate().Int64(), orgID.Int64(), userID, domain.PeriodTypeDaily, now2, now2.Add(24*time.Hour),
			domain.ScoringVersionV1EqualWeight, string(metricsJSON), string(scoresJSON), 95, now, now)

		db.Exec(`INSERT INTO finops_performance_snapshots 
			(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			node2.Generate().Int64(), orgID.Int64(), userID, domain.PeriodTypeDaily, now3, now3.Add(24*time.Hour),
			domain.ScoringVersionV1EqualWeight, string(metricsJSON), string(scoresJSON), 85, now, now)

		// We have Jan 1 (Start), Jan 2 (Start+24h), Jan 3 (Start+48h)
		// FindByUserWithLimit(limit=2) -> Should get Jan 3 and Jan 2 (DESC order)

		snaps, err := repo.FindByUserWithLimit(ctx, orgID, userID, domain.PeriodTypeDaily, start, start.Add(72*time.Hour), 2)
		assert.NoError(t, err)
		assert.Len(t, snaps, 2)
		// Expect DESC order
		assert.Equal(t, now3.Unix(), snaps[0].PeriodStart.Unix())
		assert.Equal(t, now2.Unix(), snaps[1].PeriodStart.Unix())
	})
}

// We need a dummy helper to create datatypes.JSON from string if we were mocking at struct level,
// but here we use DB.
func toJSON(v any) datatypes.JSON {
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}
