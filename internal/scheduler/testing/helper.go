// internal/scheduler/testing/helper.go
package testing

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	"gorm.io/gorm"
)

// TimeAccelerator helps speed up billing cycles for testing
type TimeAccelerator struct {
	db *gorm.DB
}

func NewTimeAccelerator(db *gorm.DB) *TimeAccelerator {
	return &TimeAccelerator{db: db}
}

// FastForwardCycle moves period_end to now for testing
func (ta *TimeAccelerator) FastForwardCycle(ctx context.Context, cycleID snowflake.ID) error {
	now := time.Now().UTC()
	return ta.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET period_end = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		now.Add(-1*time.Minute), // 1 minute ago to trigger closing
		now,
		cycleID,
		billingcycledomain.BillingCycleStatusOpen,
	).Error
}

// FastForwardAllOpenCycles speeds up all open cycles
func (ta *TimeAccelerator) FastForwardAllOpenCycles(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	result := ta.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET period_end = ?, updated_at = ?
		 WHERE status = ? AND period_end > ?`,
		now.Add(-1*time.Minute),
		now,
		billingcycledomain.BillingCycleStatusOpen,
		now,
	)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// FastForwardSubscriptionCycle moves specific subscription cycle
func (ta *TimeAccelerator) FastForwardSubscriptionCycle(ctx context.Context, subscriptionID snowflake.ID) error {
	now := time.Now().UTC()
	return ta.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET period_end = ?, updated_at = ?
		 WHERE subscription_id = ? AND status = ?`,
		now.Add(-1*time.Minute),
		now,
		subscriptionID,
		billingcycledomain.BillingCycleStatusOpen,
	).Error
}

// SetCyclePeriod allows custom period for testing
func (ta *TimeAccelerator) SetCyclePeriod(ctx context.Context, cycleID snowflake.ID, periodStart, periodEnd time.Time) error {
	return ta.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET period_start = ?, period_end = ?, updated_at = ?
		 WHERE id = ?`,
		periodStart,
		periodEnd,
		time.Now().UTC(),
		cycleID,
	).Error
}

// CycleInfo shows current cycle status for debugging
type CycleInfo struct {
	ID           snowflake.ID
	Status       billingcycledomain.BillingCycleStatus
	PeriodStart  time.Time
	PeriodEnd    time.Time
	TimeUntilEnd time.Duration
	CanClose     bool
}

func (ta *TimeAccelerator) GetCycleInfo(ctx context.Context, cycleID snowflake.ID) (*CycleInfo, error) {
	var cycle struct {
		ID          snowflake.ID
		Status      billingcycledomain.BillingCycleStatus
		PeriodStart time.Time
		PeriodEnd   time.Time
	}

	err := ta.db.WithContext(ctx).Raw(
		`SELECT id, status, period_start, period_end
		 FROM billing_cycles
		 WHERE id = ?`,
		cycleID,
	).Scan(&cycle).Error
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	info := &CycleInfo{
		ID:           cycle.ID,
		Status:       cycle.Status,
		PeriodStart:  cycle.PeriodStart,
		PeriodEnd:    cycle.PeriodEnd,
		TimeUntilEnd: cycle.PeriodEnd.Sub(now),
		CanClose:     now.After(cycle.PeriodEnd) && cycle.Status == billingcycledomain.BillingCycleStatusOpen,
	}

	return info, nil
}

// GetAllOpenCycles returns all open cycles for debugging
func (ta *TimeAccelerator) GetAllOpenCycles(ctx context.Context) ([]CycleInfo, error) {
	var cycles []struct {
		ID          snowflake.ID
		Status      billingcycledomain.BillingCycleStatus
		PeriodStart time.Time
		PeriodEnd   time.Time
	}

	err := ta.db.WithContext(ctx).Raw(
		`SELECT id, status, period_start, period_end
		 FROM billing_cycles
		 WHERE status = ?
		 ORDER BY period_end ASC`,
		billingcycledomain.BillingCycleStatusOpen,
	).Scan(&cycles).Error
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	infos := make([]CycleInfo, 0, len(cycles))
	for _, cycle := range cycles {
		infos = append(infos, CycleInfo{
			ID:           cycle.ID,
			Status:       cycle.Status,
			PeriodStart:  cycle.PeriodStart,
			PeriodEnd:    cycle.PeriodEnd,
			TimeUntilEnd: cycle.PeriodEnd.Sub(now),
			CanClose:     now.After(cycle.PeriodEnd),
		})
	}

	return infos, nil
}

// ResetCycleErrors clears error flags for retesting
func (ta *TimeAccelerator) ResetCycleErrors(ctx context.Context, cycleID snowflake.ID) error {
	now := time.Now().UTC()
	return ta.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET last_error = NULL, last_error_at = NULL, updated_at = ?
		 WHERE id = ?`,
		now,
		cycleID,
	).Error
}

// ForceReopen reopens a closed cycle (dangerous, for testing only!)
func (ta *TimeAccelerator) ForceReopen(ctx context.Context, cycleID snowflake.ID) error {
	now := time.Now().UTC()
	return ta.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET status = ?, 
		     closing_started_at = NULL,
		     rating_completed_at = NULL,
		     closed_at = NULL,
		     updated_at = ?
		 WHERE id = ?`,
		billingcycledomain.BillingCycleStatusOpen,
		now,
		cycleID,
	).Error
}
