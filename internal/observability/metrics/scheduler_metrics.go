package metrics

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/smallbiznis/valora/internal/authorization"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	"gorm.io/gorm"
)

const (
	schedulerErrorTypeDeadlineExceeded = "deadline_exceeded"
	schedulerErrorTypeAuthorization    = "authorization"
	schedulerErrorTypeBusinessRule     = "business_rule"
	schedulerErrorTypeDB               = "db"
)

const (
	SchedulerErrorTypeDeadlineExceeded = schedulerErrorTypeDeadlineExceeded
	SchedulerErrorTypeAuthorization    = schedulerErrorTypeAuthorization
	SchedulerErrorTypeBusinessRule     = schedulerErrorTypeBusinessRule
	SchedulerErrorTypeDB               = schedulerErrorTypeDB
	SchedulerErrorTypeUnknown          = "unknown"
)

const (
	SchedulerJobReasonDeadlineExceeded     = "deadline_exceeded"
	SchedulerJobReasonDBLockTimeout        = "db_lock_timeout"
	SchedulerJobReasonSerializationFailure = "serialization_failure"
	SchedulerJobReasonUniqueViolation      = "unique_violation"
	SchedulerJobReasonForbidden            = "forbidden"
	SchedulerJobReasonUnknown              = "unknown"

	SchedulerBatchDeferredReasonSkipLockedEmpty = "skip_locked_empty"
)

const (
	CycleStageCloseCycles      = "close_cycles"
	CycleStageRating           = "rating"
	CycleStageCloseAfterRating = "close_after_rating"
	CycleStageInvoice          = "invoice"
	CycleStageRecoveryRating   = "recovery_rating"
	CycleStageRecoveryClose    = "recovery_close"
	CycleStageRecoveryInvoice  = "recovery_invoice"
)

const (
	BillingCycleTransitionInvoiced = "INVOICED"
)

const (
	LockResourceSubscriptionsForWork = "subscriptions_for_work"
	LockResourceBillingCyclesForWork = "billing_cycles_for_work"
	LockResourceOpenCycle            = "billing_cycles_open_cycle"
	LockResourceBillingCycleByID     = "billing_cycle_by_id"
)

// SchedulerMetrics captures billing scheduler health signals for Cloud SLOs.
type SchedulerMetrics struct {
	jobRuns          *prometheus.CounterVec
	jobDurationV2    *prometheus.HistogramVec
	jobTimeoutsV2    *prometheus.CounterVec
	jobErrorsV2      *prometheus.CounterVec
	batchProcessedV2 *prometheus.CounterVec
	batchDeferred    *prometheus.CounterVec
	runLoopLag       prometheus.Observer
	jobDuration      *prometheus.HistogramVec
	jobTimeouts      *prometheus.CounterVec
	jobErrors        *prometheus.CounterVec
	batchProcessed   *prometheus.CounterVec
	cycleTransitions *prometheus.CounterVec
	cycleErrors      *prometheus.CounterVec
	dbLockWait       *prometheus.HistogramVec
	transitionCounts map[string]map[string]prometheus.Counter
	cycleErrorCounts map[string]map[string]prometheus.Counter
	lockWaitObserver map[string]prometheus.Observer
}

var (
	schedulerMetricsOnce sync.Once
	schedulerMetrics     *SchedulerMetrics
)

// Scheduler returns the singleton scheduler metrics registry.
func Scheduler() *SchedulerMetrics {
	return SchedulerWithConfig(Config{})
}

// SchedulerWithConfig returns the singleton scheduler metrics registry using config labels.
func SchedulerWithConfig(cfg Config) *SchedulerMetrics {
	schedulerMetricsOnce.Do(func() {
		schedulerMetrics = newSchedulerMetrics(prometheus.DefaultRegisterer, cfg)
	})
	return schedulerMetrics
}

// ResetSchedulerMetricsForTest resets the scheduler metrics singleton for tests.
func ResetSchedulerMetricsForTest() {
	schedulerMetricsOnce = sync.Once{}
	schedulerMetrics = nil
}

func newSchedulerMetrics(registerer prometheus.Registerer, cfg Config) *SchedulerMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "valora"
	}
	environment := strings.TrimSpace(cfg.Environment)
	if environment == "" {
		environment = "unknown"
	}
	constLabels := prometheus.Labels{
		"service": serviceName,
		"env":     environment,
	}

	jobRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "valora_scheduler_job_runs_total",
		Help:        "Scheduler job runs by name.",
		ConstLabels: constLabels,
	}, []string{"job"})
	jobDurationV2 := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "valora_scheduler_job_duration_seconds",
		Help:        "Scheduler job latency to protect billing batch freshness and SLOs.",
		Buckets:     []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300, 600, 1800},
		ConstLabels: constLabels,
	}, []string{"job"})
	jobTimeoutsV2 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "valora_scheduler_job_timeouts_total",
		Help:        "Scheduler job timeouts that threaten billing batch SLAs.",
		ConstLabels: constLabels,
	}, []string{"job"})
	jobErrorsV2 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "valora_scheduler_job_errors_total",
		Help:        "Scheduler job errors by low-cardinality reason.",
		ConstLabels: constLabels,
	}, []string{"job", "reason"})
	batchProcessedV2 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "valora_scheduler_batch_processed_total",
		Help:        "Scheduler batch items processed to gauge billing throughput.",
		ConstLabels: constLabels,
	}, []string{"job", "resource"})
	batchDeferred := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "valora_scheduler_batch_deferred_total",
		Help:        "Scheduler batch deferrals by low-cardinality reason.",
		ConstLabels: constLabels,
	}, []string{"job", "reason"})
	runLoopLag := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:        "valora_scheduler_runloop_lag_seconds",
		Help:        "Scheduler run loop lag beyond the configured interval.",
		Buckets:     []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		ConstLabels: constLabels,
	})

	// Tracks job latency to keep billing batches within SLA windows.
	jobDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scheduler_job_duration_seconds",
		Help:    "Scheduler job latency to protect billing batch freshness and SLOs.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300, 600, 1800},
	}, []string{"job"})
	// Highlights job timeouts that can delay revenue recognition or invoicing.
	jobTimeouts := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_job_timeout_total",
		Help: "Scheduler job timeouts that threaten billing batch SLAs.",
	}, []string{"job"})
	// Captures job failures by class for operational triage.
	jobErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_job_error_total",
		Help: "Scheduler job errors by type for billing reliability triage.",
	}, []string{"job", "error_type"})
	// Counts processed batches to understand throughput versus backlog.
	batchProcessed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_batch_processed_total",
		Help: "Scheduler batches processed to gauge billing throughput.",
	}, []string{"job"})
	// Tracks billing cycle state transitions for lifecycle integrity.
	cycleTransitions := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "billing_cycle_transition_total",
		Help: "Billing cycle lifecycle transitions to validate revenue pipeline health.",
	}, []string{"from", "to"})
	// Surfaces lifecycle errors by stage to isolate billing blockers.
	cycleErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "billing_cycle_error_total",
		Help: "Billing cycle errors by stage for faster incident isolation.",
	}, []string{"stage", "error_type"})
	// Measures lock wait time to detect contention in billing schedulers.
	dbLockWait := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scheduler_db_lock_wait_seconds",
		Help:    "Scheduler DB lock wait time for SELECT FOR UPDATE contention.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"resource"})

	registerer.MustRegister(
		jobRuns,
		jobDurationV2,
		jobTimeoutsV2,
		jobErrorsV2,
		batchProcessedV2,
		batchDeferred,
		runLoopLag,
		jobDuration,
		jobTimeouts,
		jobErrors,
		batchProcessed,
		cycleTransitions,
		cycleErrors,
		dbLockWait,
	)

	transitionCounts := map[string]map[string]prometheus.Counter{
		string(billingcycledomain.BillingCycleStatusOpen): {
			string(billingcycledomain.BillingCycleStatusClosing): cycleTransitions.WithLabelValues(
				string(billingcycledomain.BillingCycleStatusOpen),
				string(billingcycledomain.BillingCycleStatusClosing),
			),
		},
		string(billingcycledomain.BillingCycleStatusClosing): {
			string(billingcycledomain.BillingCycleStatusClosed): cycleTransitions.WithLabelValues(
				string(billingcycledomain.BillingCycleStatusClosing),
				string(billingcycledomain.BillingCycleStatusClosed),
			),
		},
		string(billingcycledomain.BillingCycleStatusClosed): {
			BillingCycleTransitionInvoiced: cycleTransitions.WithLabelValues(
				string(billingcycledomain.BillingCycleStatusClosed),
				BillingCycleTransitionInvoiced,
			),
		},
	}

	lockWaitObserver := map[string]prometheus.Observer{
		LockResourceSubscriptionsForWork: dbLockWait.WithLabelValues(LockResourceSubscriptionsForWork),
		LockResourceBillingCyclesForWork: dbLockWait.WithLabelValues(LockResourceBillingCyclesForWork),
		LockResourceOpenCycle:            dbLockWait.WithLabelValues(LockResourceOpenCycle),
		LockResourceBillingCycleByID:     dbLockWait.WithLabelValues(LockResourceBillingCycleByID),
	}

	cycleErrorCounts := map[string]map[string]prometheus.Counter{}
	errorTypes := []string{
		schedulerErrorTypeDeadlineExceeded,
		schedulerErrorTypeAuthorization,
		schedulerErrorTypeBusinessRule,
		schedulerErrorTypeDB,
	}
	for _, stage := range []string{
		CycleStageCloseCycles,
		CycleStageRating,
		CycleStageCloseAfterRating,
		CycleStageInvoice,
		CycleStageRecoveryRating,
		CycleStageRecoveryClose,
		CycleStageRecoveryInvoice,
	} {
		stageCounters := map[string]prometheus.Counter{}
		for _, errType := range errorTypes {
			stageCounters[errType] = cycleErrors.WithLabelValues(stage, errType)
		}
		cycleErrorCounts[stage] = stageCounters
	}

	return &SchedulerMetrics{
		jobRuns:          jobRuns,
		jobDurationV2:    jobDurationV2,
		jobTimeoutsV2:    jobTimeoutsV2,
		jobErrorsV2:      jobErrorsV2,
		batchProcessedV2: batchProcessedV2,
		batchDeferred:    batchDeferred,
		runLoopLag:       runLoopLag,
		jobDuration:      jobDuration,
		jobTimeouts:      jobTimeouts,
		jobErrors:        jobErrors,
		batchProcessed:   batchProcessed,
		cycleTransitions: cycleTransitions,
		cycleErrors:      cycleErrors,
		dbLockWait:       dbLockWait,
		transitionCounts: transitionCounts,
		cycleErrorCounts: cycleErrorCounts,
		lockWaitObserver: lockWaitObserver,
	}
}

// IncJobRun increments the run counter for a scheduler job.
func (m *SchedulerMetrics) IncJobRun(job string) {
	if m == nil || m.jobRuns == nil {
		return
	}
	m.jobRuns.WithLabelValues(job).Inc()
}

// ObserveJobDuration records scheduler job latency in seconds.
func (m *SchedulerMetrics) ObserveJobDuration(job string, duration time.Duration) {
	if m == nil {
		return
	}
	if m.jobDuration != nil {
		m.jobDuration.WithLabelValues(job).Observe(duration.Seconds())
	}
	if m.jobDurationV2 != nil {
		m.jobDurationV2.WithLabelValues(job).Observe(duration.Seconds())
	}
}

// IncJobTimeout increments the timeout counter for the scheduler job.
func (m *SchedulerMetrics) IncJobTimeout(job string) {
	if m == nil {
		return
	}
	if m.jobTimeouts != nil {
		m.jobTimeouts.WithLabelValues(job).Inc()
	}
	if m.jobTimeoutsV2 != nil {
		m.jobTimeoutsV2.WithLabelValues(job).Inc()
	}
}

// IncJobError increments the scheduler job error counter with classification.
func (m *SchedulerMetrics) IncJobError(job string, err error) {
	if m == nil || err == nil {
		return
	}
	if m.jobErrors != nil {
		m.jobErrors.WithLabelValues(job, classifySchedulerError(err)).Inc()
	}
	if m.jobErrorsV2 != nil {
		m.jobErrorsV2.WithLabelValues(job, ClassifySchedulerJobReason(err)).Inc()
	}
}

// IncBatchProcessed increments the batch processed counter for a job.
func (m *SchedulerMetrics) IncBatchProcessed(job string) {
	if m == nil {
		return
	}
	m.batchProcessed.WithLabelValues(job).Inc()
}

// AddBatchProcessed increments the batch processed counter for a resource by count.
func (m *SchedulerMetrics) AddBatchProcessed(job, resource string, count int) {
	if m == nil || count <= 0 || m.batchProcessedV2 == nil {
		return
	}
	m.batchProcessedV2.WithLabelValues(job, resource).Add(float64(count))
}

// IncBatchDeferred increments the batch deferred counter for a job and reason.
func (m *SchedulerMetrics) IncBatchDeferred(job, reason string) {
	if m == nil || m.batchDeferred == nil {
		return
	}
	m.batchDeferred.WithLabelValues(job, reason).Inc()
}

// ObserveRunLoopLag records lag between the scheduled tick and actual run start.
func (m *SchedulerMetrics) ObserveRunLoopLag(duration time.Duration) {
	if m == nil || m.runLoopLag == nil {
		return
	}
	lag := duration
	if lag < 0 {
		lag = 0
	}
	m.runLoopLag.Observe(lag.Seconds())
}

// IncBillingCycleTransition increments billing cycle transition counters.
func (m *SchedulerMetrics) IncBillingCycleTransition(from, to string) {
	if m == nil {
		return
	}
	if toCounters, ok := m.transitionCounts[from]; ok {
		if counter, ok := toCounters[to]; ok {
			counter.Inc()
			return
		}
	}
	m.cycleTransitions.WithLabelValues(from, to).Inc()
}

// IncBillingCycleError increments billing cycle errors by stage and type.
func (m *SchedulerMetrics) IncBillingCycleError(stage string, err error) {
	if m == nil || err == nil {
		return
	}
	errorType := classifySchedulerError(err)
	if stageCounters, ok := m.cycleErrorCounts[stage]; ok {
		if counter, ok := stageCounters[errorType]; ok {
			counter.Inc()
			return
		}
	}
	m.cycleErrors.WithLabelValues(stage, errorType).Inc()
}

// ObserveDBLockWait records lock wait time for SELECT FOR UPDATE work.
func (m *SchedulerMetrics) ObserveDBLockWait(resource string, duration time.Duration) {
	if m == nil {
		return
	}
	if observer, ok := m.lockWaitObserver[resource]; ok {
		observer.Observe(duration.Seconds())
		return
	}
	m.dbLockWait.WithLabelValues(resource).Observe(duration.Seconds())
}

func classifySchedulerError(err error) string {
	if err == nil {
		return schedulerErrorTypeBusinessRule
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return schedulerErrorTypeDeadlineExceeded
	}
	if isAuthorizationError(err) {
		return schedulerErrorTypeAuthorization
	}
	if isDBError(err) {
		return schedulerErrorTypeDB
	}
	return schedulerErrorTypeBusinessRule
}

// ClassifySchedulerErrorType returns a low-cardinality error type for logging.
func ClassifySchedulerErrorType(err error) string {
	if err == nil {
		return SchedulerErrorTypeUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return SchedulerErrorTypeDeadlineExceeded
	}
	if isAuthorizationError(err) {
		return SchedulerErrorTypeAuthorization
	}
	if isDBError(err) {
		return SchedulerErrorTypeDB
	}
	return SchedulerErrorTypeBusinessRule
}

// IsSchedulerErrorRetryable reports whether the scheduler error should be retried.
func IsSchedulerErrorRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	return isDBError(err)
}

// ClassifySchedulerJobReason maps scheduler job errors to low-cardinality reasons.
func ClassifySchedulerJobReason(err error) string {
	return classifySchedulerJobReason(err)
}

func classifySchedulerJobReason(err error) string {
	if err == nil {
		return SchedulerJobReasonUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return SchedulerJobReasonDeadlineExceeded
	}
	if isAuthorizationError(err) {
		return SchedulerJobReasonForbidden
	}
	if isDBLockTimeout(err) {
		return SchedulerJobReasonDBLockTimeout
	}
	if isSerializationFailure(err) {
		return SchedulerJobReasonSerializationFailure
	}
	if isUniqueViolation(err) {
		return SchedulerJobReasonUniqueViolation
	}
	return SchedulerJobReasonUnknown
}

func isDBLockTimeout(err error) bool {
	return hasPGCode(err, "55P03")
}

func isSerializationFailure(err error) bool {
	return hasPGCode(err, "40001")
}

func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return hasPGCode(err, "23505")
}

func hasPGCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}

func isAuthorizationError(err error) bool {
	return errors.Is(err, authorization.ErrForbidden) ||
		errors.Is(err, authorization.ErrInvalidActor) ||
		errors.Is(err, authorization.ErrInvalidOrganization) ||
		errors.Is(err, authorization.ErrInvalidObject) ||
		errors.Is(err, authorization.ErrInvalidAction)
}

func isDBError(err error) bool {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	if errors.Is(err, gorm.ErrInvalidDB) ||
		errors.Is(err, gorm.ErrInvalidTransaction) ||
		errors.Is(err, gorm.ErrInvalidField) ||
		errors.Is(err, gorm.ErrInvalidData) ||
		errors.Is(err, gorm.ErrMissingWhereClause) ||
		errors.Is(err, gorm.ErrUnsupportedDriver) ||
		errors.Is(err, gorm.ErrRegistered) ||
		errors.Is(err, gorm.ErrInvalidValue) ||
		errors.Is(err, gorm.ErrNotImplemented) ||
		errors.Is(err, gorm.ErrDryRunModeUnsupported) ||
		errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr)
}
