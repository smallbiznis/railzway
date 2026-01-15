package service

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/glebarez/sqlite"
	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/billingoperations/repository"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"gorm.io/gorm"
)

func TestServiceReadAPI(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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

	repo := repository.NewRepository(db)
	svc := &Service{
		db:           db,
		log:          zaptest.NewLogger(t),
		clock:        &clock.SystemClock{},
		repo:         repo,
		billingCfg:   &config.BillingConfigHolder{},
	}

	node, _ := snowflake.NewNode(1)
	orgID := node.Generate()
	userID := "user_svc_test"

	// Seed data
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	metricsJSON := `{"total_assigned": 10, "total_resolved": 8, "total_escalated": 1, "avg_response_ms": 1800000, "exposure_handled": 1000}`
	scoresJSON := `{"total": 80}`

	db.Exec(`INSERT INTO finops_performance_snapshots 
		(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.Generate().Int64(), orgID.Int64(), userID, domain.PeriodTypeDaily, start, end,
		domain.ScoringVersionV1EqualWeight, metricsJSON, scoresJSON, 80, now, now)

	// Another user same org
	user2 := "user_svc_test_2"
	// User 2 has 2 snapshots
	scoresJSON90 := `{"total": 90}`
	db.Exec(`INSERT INTO finops_performance_snapshots 
		(id, org_id, user_id, period_type, period_start, period_end, scoring_version, metrics, scores, total_score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.Generate().Int64(), orgID.Int64(), user2, domain.PeriodTypeDaily, start, end,
		domain.ScoringVersionV1EqualWeight, metricsJSON, scoresJSON90, 90, now, now)

	ctx := orgcontext.WithOrgID(context.Background(), orgID.Int64())

	t.Run("GetMyPerformance", func(t *testing.T) {
		req := domain.GetPerformanceRequest{
			PeriodType: domain.PeriodTypeDaily,
			From:       start,
			To:         end.Add(24 * time.Hour),
			Limit:      10,
		}
		resp, err := svc.GetMyPerformance(ctx, userID, req)
		assert.NoError(t, err)
		assert.Equal(t, userID, resp.UserID)
		assert.Len(t, resp.Snapshots, 1) // Only 1 specific to userID
	})

	t.Run("GetTeamPerformance_Aggregation", func(t *testing.T) {
		req := domain.GetPerformanceRequest{
			PeriodType: domain.PeriodTypeDaily, // Though we are aggregating daily snapshots
			From:       start,
			To:         end.Add(24 * time.Hour),
		}
		resp, err := svc.GetTeamPerformance(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, 2, resp.TeamSize)
		assert.Len(t, resp.Snapshots, 2)

		// Check aggregation logic
		// user_svc_test: 1 snapshot, score 80
		// user_svc_test_2: 1 snapshot, score 90

		for _, s := range resp.Snapshots {
			if s.UserID == userID {
				assert.Equal(t, 80, s.AvgScore)
				assert.Equal(t, 0.8, s.MetricsSummary.CompletionRatio) // 8/10
				// 1800000 ms = 30 min
				assert.Equal(t, 30.0, s.MetricsSummary.AvgResponseMinutes)
			} else if s.UserID == user2 {
				assert.Equal(t, 90, s.AvgScore)
			}
		}
	})
}
