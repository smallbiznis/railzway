package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/smallbiznis/railzway/internal/authorization"
	"gorm.io/gorm"
)

func TestClassifySchedulerJobReason(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "deadline",
			err:  context.DeadlineExceeded,
			want: SchedulerJobReasonDeadlineExceeded,
		},
		{
			name: "forbidden",
			err:  authorization.ErrForbidden,
			want: SchedulerJobReasonForbidden,
		},
		{
			name: "db_lock_timeout",
			err:  &pgconn.PgError{Code: "55P03"},
			want: SchedulerJobReasonDBLockTimeout,
		},
		{
			name: "serialization_failure",
			err:  &pgconn.PgError{Code: "40001"},
			want: SchedulerJobReasonSerializationFailure,
		},
		{
			name: "unique_violation",
			err:  gorm.ErrDuplicatedKey,
			want: SchedulerJobReasonUniqueViolation,
		},
		{
			name: "unknown",
			err:  errors.New("boom"),
			want: SchedulerJobReasonUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifySchedulerJobReason(tc.err); got != tc.want {
				t.Fatalf("expected reason %q, got %q", tc.want, got)
			}
		})
	}
}

func TestAddBatchProcessed(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newSchedulerMetrics(registry, Config{
		ServiceName: "railzway",
		Environment: "test",
	})

	metrics.AddBatchProcessed("ensure_cycles", "subscriptions", 3)

	got := testutil.ToFloat64(metrics.batchProcessedV2.WithLabelValues("ensure_cycles", "subscriptions"))
	if got != 3 {
		t.Fatalf("expected processed count 3, got %v", got)
	}
}
