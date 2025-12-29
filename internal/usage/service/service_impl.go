package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/cloudmetrics"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
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
)

type ServiceParam struct {
	fx.In

	DB       *gorm.DB
	Log      *zap.Logger
	GenID    *snowflake.Node
	MeterSvc meterdomain.Service
	SubSvc   subscriptiondomain.Service
	Metrics  *cloudmetrics.CloudMetrics
}

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID     *snowflake.Node
	metersvc  meterdomain.Service
	subSvc    subscriptiondomain.Service
	usagerepo repository.Repository[usagedomain.UsageEvent]
	metrics   *cloudmetrics.CloudMetrics
}

func NewService(p ServiceParam) usagedomain.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("usage.service"),

		genID:     p.GenID,
		metersvc:  p.MeterSvc,
		subSvc:    p.SubSvc,
		usagerepo: repository.ProvideStore[usagedomain.UsageEvent](p.DB),
		metrics:   p.Metrics,
	}
}

func (s *Service) Ingest(ctx context.Context, req usagedomain.CreateIngestRequest) (*usagedomain.UsageEvent, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	customerID, err := s.parseID(req.CustomerID, usagedomain.ErrInvalidCustomer)
	if err != nil {
		return nil, err
	}

	meterCode := strings.TrimSpace(req.MeterCode)
	if meterCode == "" {
		return nil, usagedomain.ErrInvalidMeterCode
	}

	meter, err := s.resolveMeter(ctx, meterCode)
	if err != nil {
		return nil, err
	}

	subscription, err := s.resolveActiveSubscription(ctx, req.CustomerID)
	if err != nil {
		return nil, err
	}

	subscriptionItem, err := s.resolveSubscriptionItem(ctx, subscription.ID.String(), meter.ID)
	if err != nil {
		return nil, err
	}

	meterID, err := s.parseID(meter.ID, usagedomain.ErrInvalidMeter)
	if err != nil {
		return nil, err
	}

	if subscriptionItem.MeterID == nil || *subscriptionItem.MeterID != meterID {
		return nil, usagedomain.ErrInvalidMeter
	}

	if err := validateUsageEvent(req); err != nil {
		return nil, err
	}

	idempotencyKey := normalizeIdempotencyKey(req.IdempotencyKey)

	record := &usagedomain.UsageEvent{
		ID:                 s.genID.Generate(),
		OrgID:              orgID,
		CustomerID:         customerID,
		SubscriptionID:     subscription.ID,
		SubscriptionItemID: subscriptionItem.ID,
		MeterID:            meterID,
		MeterCode:          meterCode,
		Value:              req.Value,
		RecordedAt:         req.RecordedAt,
		IdempotencyKey:     idempotencyKey,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	if req.Metadata != nil {
		record.Metadata = datatypes.JSONMap(req.Metadata)
	}

	if err := s.usagerepo.Create(ctx, record); err != nil {
		return nil, err
	}

	if s.metrics != nil {
		// Cloud accounting metric: emitted usage events are not billing inputs.
		s.metrics.IncUsageEvent(orgID.String(), meterCode)
	}
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

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	if orgID, ok := orgcontext.OrgIDFromContext(ctx); ok && orgID != 0 {
		return snowflake.ID(orgID), nil
	}

	if raw := ctx.Value("org_id"); raw != nil {
		switch value := raw.(type) {
		case int64:
			if value != 0 {
				return snowflake.ID(value), nil
			}
		case snowflake.ID:
			if value != 0 {
				return value, nil
			}
		case string:
			parsed, err := snowflake.ParseString(strings.TrimSpace(value))
			if err == nil && parsed != 0 {
				return parsed, nil
			}
		}
	}

	return 0, usagedomain.ErrInvalidOrganization
}

func (s *Service) resolveMeter(ctx context.Context, meterCode string) (*meterdomain.Response, error) {
	meter, err := s.metersvc.GetByCode(ctx, meterCode)
	if err != nil {
		switch {
		case errors.Is(err, meterdomain.ErrInvalidCode), errors.Is(err, meterdomain.ErrNotFound):
			return nil, usagedomain.ErrInvalidMeterCode
		default:
			return nil, err
		}
	}
	return meter, nil
}

func (s *Service) resolveActiveSubscription(ctx context.Context, customerID string) (subscriptiondomain.Subscription, error) {
	subscription, err := s.subSvc.GetActiveByCustomerID(ctx, subscriptiondomain.GetActiveByCustomerIDRequest{
		CustomerID: customerID,
	})
	if err != nil {
		switch {
		case errors.Is(err, subscriptiondomain.ErrSubscriptionNotFound):
			return subscriptiondomain.Subscription{}, usagedomain.ErrInvalidSubscription
		case errors.Is(err, subscriptiondomain.ErrInvalidCustomer):
			return subscriptiondomain.Subscription{}, usagedomain.ErrInvalidCustomer
		default:
			return subscriptiondomain.Subscription{}, err
		}
	}
	return subscription, nil
}

func (s *Service) resolveSubscriptionItem(ctx context.Context, subscriptionID, meterID string) (subscriptiondomain.SubscriptionItem, error) {
	item, err := s.subSvc.GetSubscriptionItem(ctx, subscriptiondomain.GetSubscriptionItemRequest{
		SubscriptionID: subscriptionID,
		MeterID:        meterID,
	})
	if err != nil {
		switch {
		case errors.Is(err, subscriptiondomain.ErrSubscriptionItemNotFound):
			return subscriptiondomain.SubscriptionItem{}, usagedomain.ErrInvalidSubscriptionItem
		case errors.Is(err, subscriptiondomain.ErrInvalidMeterID):
			return subscriptiondomain.SubscriptionItem{}, usagedomain.ErrInvalidMeter
		case errors.Is(err, subscriptiondomain.ErrInvalidMeterCode):
			return subscriptiondomain.SubscriptionItem{}, usagedomain.ErrInvalidMeterCode
		case errors.Is(err, subscriptiondomain.ErrInvalidSubscription):
			return subscriptiondomain.SubscriptionItem{}, usagedomain.ErrInvalidSubscription
		default:
			return subscriptiondomain.SubscriptionItem{}, err
		}
	}
	return item, nil
}

func validateUsageEvent(req usagedomain.CreateIngestRequest) error {
	if req.RecordedAt.IsZero() {
		return usagedomain.ErrInvalidRecordedAt
	}
	if math.IsNaN(req.Value) || math.IsInf(req.Value, 0) {
		return usagedomain.ErrInvalidValue
	}
	return nil
}

func normalizeIdempotencyKey(key *string) *string {
	if key == nil {
		return nil
	}
	value := strings.TrimSpace(*key)
	if value == "" {
		return nil
	}
	return &value
}

func (s *Service) buildUsageFilter(ctx context.Context, req usagedomain.ListUsageRequest) (*usagedomain.UsageEvent, int32, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return nil, 0, err
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
