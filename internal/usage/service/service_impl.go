package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/cache"
	"github.com/smallbiznis/valora/internal/cloudmetrics"
	"github.com/smallbiznis/valora/internal/events"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	"github.com/smallbiznis/valora/internal/orgcontext"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	usagedomain "github.com/smallbiznis/valora/internal/usage/domain"
	"github.com/smallbiznis/valora/pkg/db/option"
	"github.com/smallbiznis/valora/pkg/db/pagination"
	"github.com/smallbiznis/valora/pkg/repository"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ServiceParam struct {
	fx.In

	DB            *gorm.DB
	Log           *zap.Logger
	GenID         *snowflake.Node
	MeterSvc      meterdomain.Service
	SubSvc        subscriptiondomain.Service
	Metrics       *cloudmetrics.CloudMetrics
	ObsMetrics    *obsmetrics.Metrics `optional:"true"`
	ResolverCache cache.UsageResolverCache
	Outbox        *events.Outbox `optional:"true"`
}

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID         *snowflake.Node
	metersvc      meterdomain.Service
	subSvc        subscriptiondomain.Service
	usagerepo     repository.Repository[usagedomain.UsageEvent]
	metrics       *cloudmetrics.CloudMetrics
	obsMetrics    *obsmetrics.Metrics
	resolverCache cache.UsageResolverCache
	outbox        *events.Outbox
}

func NewService(p ServiceParam) usagedomain.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("usage.service"),

		genID:         p.GenID,
		metersvc:      p.MeterSvc,
		subSvc:        p.SubSvc,
		usagerepo:     repository.ProvideStore[usagedomain.UsageEvent](p.DB),
		metrics:       p.Metrics,
		obsMetrics:    p.ObsMetrics,
		resolverCache: p.ResolverCache,
		outbox:        p.Outbox,
	}
}

func (s *Service) Ingest(
	ctx context.Context,
	req usagedomain.CreateIngestRequest,
) (*usagedomain.UsageEvent, error) {

	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, usagedomain.ErrInvalidOrganization
	}

	customerID, err := s.parseID(req.CustomerID, usagedomain.ErrInvalidCustomer)
	if err != nil {
		return nil, err
	}

	meterCode := strings.TrimSpace(req.MeterCode)
	if meterCode == "" {
		return nil, usagedomain.ErrInvalidMeterCode
	}

	if err := validateUsageEvent(req); err != nil {
		return nil, err
	}

	if err := s.ensureCustomerExists(ctx, orgID, customerID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	recordedAt := req.RecordedAt
	if recordedAt.IsZero() {
		recordedAt = now
	}

	idempotencyKey := normalizeIdempotencyKey(req.IdempotencyKey)

	record := &usagedomain.UsageEvent{
		ID:             s.genID.Generate(),
		OrgID:          orgID,
		CustomerID:     customerID,
		MeterCode:      meterCode,
		Value:          req.Value,
		RecordedAt:     recordedAt,
		Status:         usagedomain.UsageStatusAccepted,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if req.Metadata != nil {
		record.Metadata = datatypes.JSONMap(req.Metadata)
	}

	inserted, err := s.insertUsageEvent(ctx, record, idempotencyKey)
	if err != nil {
		return nil, err
	}

	// 🔁 Idempotency hit → fetch existing
	if !inserted && idempotencyKey != "" {
		existing, err := s.findUsageEventByIdempotencyKey(
			ctx,
			orgID,
			idempotencyKey,
		)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	// async metrics (best effort)
	if s.metrics != nil {
		go s.metrics.IncUsageEvent(orgID.String(), meterCode)
	}

	if s.obsMetrics != nil {
		s.obsMetrics.RecordUsageIngest(ctx, meterCode)
	}

	s.emitUsageIngested(record)

	return record, nil
}

func (s *Service) List(ctx context.Context, req usagedomain.ListUsageRequest) (usagedomain.ListUsageResponse, error) {
	filter, pageSize, err := s.buildUsageFilter(ctx, req)
	if err != nil {
		return usagedomain.ListUsageResponse{}, err
	}

	items, err := s.usagerepo.Find(ctx, filter,
		option.ApplyPagination(pagination.Pagination{
			PageToken: req.PageToken,
			PageSize:  int(pageSize),
		}),
		option.WithSortBy(option.QuerySortBy{Allow: map[string]bool{"created_at": true}}),
	)
	if err != nil {
		return usagedomain.ListUsageResponse{}, err
	}
	return buildUsageListResponse(items, pageSize)
}

func (s *Service) parseID(value string, invalidErr error) (snowflake.ID, error) {
	id, err := snowflake.ParseString(strings.TrimSpace(value))
	if err != nil || id == 0 {
		return 0, invalidErr
	}
	return id, nil
}

func parseOptionalID(value string) snowflake.ID {
	id, err := snowflake.ParseString(strings.TrimSpace(value))
	if err != nil || id == 0 {
		return 0
	}
	return id
}

func (s *Service) resolveMeter(ctx context.Context, orgID snowflake.ID, meterCode string) (*meterdomain.Response, error) {
	if s.resolverCache != nil {
		if cached, ok := s.resolverCache.GetMeter(orgID.String(), meterCode); ok {
			return cached, nil
		}
	}
	if s.metersvc == nil {
		return nil, nil
	}
	meter, err := s.metersvc.GetByCode(ctx, meterCode)
	if err != nil {
		switch {
		case errors.Is(err, meterdomain.ErrInvalidCode):
			return nil, usagedomain.ErrInvalidMeterCode
		case errors.Is(err, meterdomain.ErrMeterNotFound):
			return nil, nil
		default:
			return nil, nil
		}
	}
	if s.resolverCache != nil {
		s.resolverCache.SetMeter(orgID.String(), meterCode, meter)
	}
	return meter, nil
}

func (s *Service) resolveActiveSubscription(ctx context.Context, orgID snowflake.ID, customerID string) (subscriptiondomain.Subscription, error) {
	if s.resolverCache != nil {
		if cached, ok := s.resolverCache.GetActiveSubscription(orgID.String(), customerID); ok {
			return cached, nil
		}
	}
	if s.subSvc == nil {
		return subscriptiondomain.Subscription{}, nil
	}
	subscription, err := s.subSvc.GetActiveByCustomerID(ctx, subscriptiondomain.GetActiveByCustomerIDRequest{
		CustomerID: customerID,
	})
	if err != nil {
		switch {
		case errors.Is(err, subscriptiondomain.ErrSubscriptionNotFound):
			return subscriptiondomain.Subscription{}, nil
		case errors.Is(err, subscriptiondomain.ErrInvalidCustomer):
			return subscriptiondomain.Subscription{}, usagedomain.ErrInvalidCustomer
		default:
			return subscriptiondomain.Subscription{}, nil
		}
	}
	if s.resolverCache != nil {
		s.resolverCache.SetActiveSubscription(orgID.String(), customerID, subscription)
	}
	return subscription, nil
}

func (s *Service) resolveSubscriptionItem(ctx context.Context, subscriptionID, meterID string) (subscriptiondomain.SubscriptionItem, error) {
	if subscriptionID == "" || meterID == "" {
		return subscriptiondomain.SubscriptionItem{}, nil
	}
	if s.resolverCache != nil {
		if cached, ok := s.resolverCache.GetSubscriptionItem(subscriptionID, meterID); ok {
			return cached, nil
		}
	}
	if s.subSvc == nil {
		return subscriptiondomain.SubscriptionItem{}, nil
	}
	item, err := s.subSvc.GetSubscriptionItem(ctx, subscriptiondomain.GetSubscriptionItemRequest{
		SubscriptionID: subscriptionID,
		MeterID:        meterID,
	})
	if err != nil {
		return subscriptiondomain.SubscriptionItem{}, nil
	}
	if s.resolverCache != nil {
		s.resolverCache.SetSubscriptionItem(subscriptionID, meterID, item)
	}
	return item, nil
}

func (s *Service) ensureCustomerExists(ctx context.Context, orgID, customerID snowflake.ID) error {
	if s.db == nil {
		return errors.New("missing_db")
	}
	var exists bool
	if err := s.db.WithContext(ctx).Raw(
		`SELECT EXISTS(SELECT 1 FROM customers WHERE org_id = ? AND id = ?)`,
		orgID,
		customerID,
	).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return usagedomain.ErrInvalidCustomer
	}
	return nil
}

func (s *Service) insertUsageEvent(ctx context.Context, record *usagedomain.UsageEvent, idempotencyKey string) (bool, error) {
	if record == nil {
		return false, errors.New("missing_usage_event")
	}
	if s.db == nil {
		return false, errors.New("missing_db")
	}
	if strings.EqualFold(s.db.Dialector.Name(), "sqlite") {
		return s.insertUsageEventSQLite(ctx, record, idempotencyKey)
	}
	db := s.db.WithContext(ctx)
	if idempotencyKey != "" {
		db = db.Clauses(buildIdempotencyConflictClause(s.db))
	}
	result := db.Create(record)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *Service) insertUsageEventSQLite(ctx context.Context, record *usagedomain.UsageEvent, idempotencyKey string) (bool, error) {
	var subscriptionItemValue any
	if record.SubscriptionItemID != 0 {
		subscriptionItemValue = record.SubscriptionItemID
	}
	query := `INSERT INTO usage_events (
		id, org_id, customer_id, subscription_id, subscription_item_id,
		meter_id, meter_code, value, recorded_at, status, error,
		idempotency_key, metadata, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if idempotencyKey != "" {
		query += " ON CONFLICT (org_id, idempotency_key) DO NOTHING"
	}
	result := s.db.WithContext(ctx).Exec(
		query,
		record.ID,
		record.OrgID,
		record.CustomerID,
		record.SubscriptionID,
		subscriptionItemValue,
		record.MeterID,
		record.MeterCode,
		record.Value,
		record.RecordedAt,
		record.Status,
		record.Error,
		idempotencyKey,
		record.Metadata,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *Service) findUsageEventByIdempotencyKey(ctx context.Context, orgID snowflake.ID, key string) (*usagedomain.UsageEvent, error) {
	if s.db == nil {
		return nil, errors.New("missing_db")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var record usagedomain.UsageEvent
	err := s.db.WithContext(ctx).
		Where("org_id = ? AND idempotency_key = ?", orgID, key).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *Service) emitUsageIngested(record *usagedomain.UsageEvent) {
	if s.outbox == nil || record == nil {
		return
	}
	payload := events.UsageIngestedPayload{
		UsageEventID: record.ID.String(),
		OrgID:        record.OrgID.String(),
		CustomerID:   record.CustomerID.String(),
		MeterCode:    record.MeterCode,
	}
	if record.SubscriptionID != 0 {
		payload.SubscriptionID = record.SubscriptionID.String()
	}
	if record.SubscriptionItemID != 0 {
		payload.SubscriptionItemID = record.SubscriptionItemID.String()
	}
	if record.MeterID != 0 {
		payload.MeterID = record.MeterID.String()
	}
	if record.IdempotencyKey != "" {
		payload.IdempotencyKey = record.IdempotencyKey
	}
	event := events.Event{
		OrgID:     record.OrgID,
		Type:      events.EventUsageIngested,
		Payload:   payload.ToMap(),
		DedupeKey: record.ID.String(),
	}
	go func() {
		_ = s.outbox.Publish(context.Background(), event)
	}()
}

func buildIdempotencyConflictClause(db *gorm.DB) clause.OnConflict {
	conflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "idempotency_key"}},
		DoNothing: true,
	}
	if db != nil && strings.EqualFold(db.Dialector.Name(), "postgres") {
		conflict.TargetWhere = clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "idempotency_key IS NOT NULL"},
		}}
	}
	return conflict
}

func validateUsageEvent(req usagedomain.CreateIngestRequest) error {
	if math.IsNaN(req.Value) || math.IsInf(req.Value, 0) {
		return usagedomain.ErrInvalidValue
	}
	if req.RecordedAt.IsZero() {
		return usagedomain.ErrInvalidRecordedAt
	}
	if req.IdempotencyKey == "" {
		return usagedomain.ErrInvalidIdempotencyKey
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return usagedomain.ErrInvalidIdempotencyKey
	}
	return nil
}

func normalizeIdempotencyKey(key string) string {
	value := strings.TrimSpace(key)
	if value == "" {
		return ""
	}

	if value == key {
		return key
	}
	return value
}

func (s *Service) buildUsageFilter(ctx context.Context, req usagedomain.ListUsageRequest) (*usagedomain.UsageEvent, int32, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return nil, 0, usagedomain.ErrInvalidOrganization
	}

	filter := &usagedomain.UsageEvent{
		OrgID: orgID,
	}

	if req.CustomerID != "" {
		customerID, err := s.parseID(req.CustomerID, usagedomain.ErrInvalidCustomer)
		if err != nil {
			return nil, 0, err
		}
		filter.CustomerID = customerID
	}

	if req.SubscriptionID != "" {
		subscriptionID, err := s.parseID(req.SubscriptionID, usagedomain.ErrInvalidSubscription)
		if err != nil {
			return nil, 0, err
		}
		filter.SubscriptionID = subscriptionID
	}

	if req.MeterID != "" {
		meterID, err := s.parseID(req.MeterID, usagedomain.ErrInvalidMeter)
		if err != nil {
			return nil, 0, err
		}
		filter.MeterID = meterID
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	return filter, pageSize, nil
}

func buildUsageListResponse(items []*usagedomain.UsageEvent, pageSize int32) (usagedomain.ListUsageResponse, error) {
	pageInfo := pagination.BuildCursorPageInfo(items, pageSize, func(record *usagedomain.UsageEvent) string {
		token, err := pagination.EncodeCursor(pagination.Cursor{
			ID:        record.ID.String(),
			CreatedAt: record.CreatedAt.Format(time.RFC3339),
		})
		if err != nil {
			return ""
		}
		return token
	})
	if pageInfo != nil && pageInfo.HasMore && len(items) > int(pageSize) {
		items = items[:pageSize]
	}

	records := make([]usagedomain.UsageEvent, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		records = append(records, *item)
	}

	resp := usagedomain.ListUsageResponse{
		UsageEvents: records,
	}
	if pageInfo != nil {
		resp.PageInfo = *pageInfo
	}

	return resp, nil
}
