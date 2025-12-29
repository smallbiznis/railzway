package service

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	"github.com/smallbiznis/valora/internal/orgcontext"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"github.com/smallbiznis/valora/pkg/db/option"
	"github.com/smallbiznis/valora/pkg/db/pagination"
	"github.com/smallbiznis/valora/pkg/repository"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	log *zap.Logger

	genID            *snowflake.Node
	repo             subscriptiondomain.Repository
	billingCycleRepo repository.Repository[billingcycledomain.BillingCycle]
	subscriptionRepo repository.Repository[subscriptiondomain.Subscription]

	pricesvc pricedomain.Service
}

type ServiceParam struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
	Repo  subscriptiondomain.Repository

	Pricesvc pricedomain.Service
}

func NewService(p ServiceParam) subscriptiondomain.Service {
	return &Service{
		db:  p.DB,
		log: p.Log.Named("subscription.service"),

		genID:            p.GenID,
		repo:             p.Repo,
		billingCycleRepo: repository.ProvideStore[billingcycledomain.BillingCycle](p.DB),
		subscriptionRepo: repository.ProvideStore[subscriptiondomain.Subscription](p.DB),

		pricesvc: p.Pricesvc,
	}
}

// GetActiveByCustomerID implements domain.Service.
func (s *Service) GetActiveByCustomerID(ctx context.Context, req subscriptiondomain.GetActiveByCustomerIDRequest) (subscriptiondomain.Subscription, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return subscriptiondomain.Subscription{}, err
	}

	customerID, err := s.parseID(req.CustomerID, subscriptiondomain.ErrInvalidCustomer)
	if err != nil {
		return subscriptiondomain.Subscription{}, err
	}

	statuses := []subscriptiondomain.SubscriptionStatus{
		subscriptiondomain.SubscriptionStatusActive,
	}

	item, err := s.repo.FindActiveByCustomerID(ctx, s.db, orgID, customerID, statuses)
	if err != nil {
		return subscriptiondomain.Subscription{}, err
	}
	if item == nil {
		return subscriptiondomain.Subscription{}, subscriptiondomain.ErrSubscriptionNotFound
	}

	return *item, nil
}

// GetSubscriptionItem implements domain.Service.
func (s *Service) GetSubscriptionItem(ctx context.Context, req subscriptiondomain.GetSubscriptionItemRequest) (subscriptiondomain.SubscriptionItem, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return subscriptiondomain.SubscriptionItem{}, err
	}

	subscriptionID, err := s.parseID(req.SubscriptionID, subscriptiondomain.ErrInvalidSubscription)
	if err != nil {
		return subscriptiondomain.SubscriptionItem{}, err
	}

	meterID := strings.TrimSpace(req.MeterID)
	if meterID != "" {
		parsedMeterID, err := s.parseID(meterID, subscriptiondomain.ErrInvalidMeterID)
		if err != nil {
			return subscriptiondomain.SubscriptionItem{}, err
		}

		item, err := s.repo.FindSubscriptionItemByMeterID(ctx, s.db, orgID, subscriptionID, parsedMeterID)
		if err != nil {
			return subscriptiondomain.SubscriptionItem{}, err
		}
		if item == nil {
			return subscriptiondomain.SubscriptionItem{}, subscriptiondomain.ErrSubscriptionItemNotFound
		}

		return *item, nil
	}

	meterCode := strings.TrimSpace(req.MeterCode)
	if meterCode == "" {
		return subscriptiondomain.SubscriptionItem{}, subscriptiondomain.ErrInvalidMeterCode
	}

	item, err := s.repo.FindSubscriptionItemByMeterCode(ctx, s.db, orgID, subscriptionID, meterCode)
	if err != nil {
		return subscriptiondomain.SubscriptionItem{}, err
	}
	if item == nil {
		return subscriptiondomain.SubscriptionItem{}, subscriptiondomain.ErrSubscriptionItemNotFound
	}

	return *item, nil
}

func (s *Service) List(ctx context.Context, req subscriptiondomain.ListSubscriptionRequest) (subscriptiondomain.ListSubscriptionResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return subscriptiondomain.ListSubscriptionResponse{}, err
	}

	filter := &subscriptiondomain.Subscription{
		OrgID: orgID,
	}

	statusFilter, err := parseStatusFilter(req.Status)
	if err != nil {
		return subscriptiondomain.ListSubscriptionResponse{}, err
	}
	if statusFilter != nil {
		filter.Status = *statusFilter
	}

	if req.CustomerID != "" {
		customerID, err := s.parseID(req.CustomerID, subscriptiondomain.ErrInvalidCustomer)
		if err != nil {
			return subscriptiondomain.ListSubscriptionResponse{}, err
		}
		filter.CustomerID = customerID
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	options := []option.QueryOption{
		option.ApplyPagination(pagination.Pagination{
			PageToken: req.PageToken,
			PageSize:  int(pageSize),
		}),
		option.WithSortBy(option.WithQuerySortBy("created_at", "desc", map[string]bool{"created_at": true})),
	}

	if req.CreatedFrom != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "created_at",
			Operator: option.GTE,
			Value:    *req.CreatedFrom,
		}))
	}
	if req.CreatedTo != nil {
		options = append(options, option.ApplyOperator(option.Condition{
			Field:    "created_at",
			Operator: option.LTE,
			Value:    *req.CreatedTo,
		}))
	}

	items, err := s.subscriptionRepo.Find(ctx, filter, options...)
	if err != nil {
		return subscriptiondomain.ListSubscriptionResponse{}, err
	}

	pageInfo := pagination.BuildCursorPageInfo(items, pageSize, func(item *subscriptiondomain.Subscription) string {
		token, err := pagination.EncodeCursor(pagination.Cursor{
			ID:        item.ID.String(),
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
		})
		if err != nil {
			return ""
		}
		return token
	})
	if pageInfo != nil && pageInfo.HasMore && len(items) > int(pageSize) {
		items = items[:pageSize]
	}

	subscriptions := make([]subscriptiondomain.Subscription, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		subscriptions = append(subscriptions, *item)
	}

	resp := subscriptiondomain.ListSubscriptionResponse{
		Subscriptions: subscriptions,
	}
	if pageInfo != nil {
		resp.PageInfo = *pageInfo
	}

	return resp, nil
}

func (s *Service) Create(ctx context.Context, req subscriptiondomain.CreateSubscriptionRequest) (subscriptiondomain.CreateSubscriptionResponse, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return subscriptiondomain.CreateSubscriptionResponse{}, err
	}

	customerID, err := s.parseID(req.CustomerID, subscriptiondomain.ErrInvalidCustomer)
	if err != nil {
		return subscriptiondomain.CreateSubscriptionResponse{}, err
	}

	if len(req.Items) == 0 {
		return subscriptiondomain.CreateSubscriptionResponse{}, subscriptiondomain.ErrInvalidItems
	}

	now := time.Now().UTC()
	subscription := subscriptiondomain.Subscription{
		ID:             s.genID.Generate(),
		OrgID:          orgID,
		CustomerID:     customerID,
		Status:         subscriptiondomain.SubscriptionStatusDraft,
		CollectionMode: req.CollectionMode,
		StartAt:        now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if req.Metadata != nil {
		subscription.Metadata = datatypes.JSONMap(req.Metadata)
	}

	subscriptionItems, billingCycleType, err := s.buildSubscriptionItems(ctx, orgID, subscription.ID, req.Items, now)
	if err != nil {
		return subscriptiondomain.CreateSubscriptionResponse{}, err
	}
	subscription.BillingCycleType = billingCycleType

	if err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.repo.Insert(ctx, tx, &subscription); err != nil {
			return err
		}
		if err := s.repo.InsertItems(ctx, tx, subscriptionItems); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return subscriptiondomain.CreateSubscriptionResponse{}, err
	}

	return s.toCreateResponse(&subscription, subscriptionItems), nil
}

func (s *Service) GetByID(ctx context.Context, id string) (subscriptiondomain.Subscription, error) {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return subscriptiondomain.Subscription{}, err
	}

	subscriptionID, err := snowflake.ParseString(strings.TrimSpace(id))
	if err != nil {
		return subscriptiondomain.Subscription{}, err
	}

	item, err := s.repo.FindByID(ctx, s.db, orgID, subscriptionID)
	if err != nil {
		return subscriptiondomain.Subscription{}, err
	}
	if item == nil {
		return subscriptiondomain.Subscription{}, gorm.ErrRecordNotFound
	}

	return *item, nil
}

func (s *Service) TransitionSubscription(
	ctx context.Context,
	subscriptionID string,
	targetStatus subscriptiondomain.SubscriptionStatus,
	reason subscriptiondomain.TransitionReason,
) error {
	orgID, err := s.orgIDFromContext(ctx)
	if err != nil {
		return err
	}

	_ = reason

	id, err := s.parseID(subscriptionID, subscriptiondomain.ErrInvalidSubscription)
	if err != nil {
		return err
	}

	if !isValidStatus(targetStatus) {
		return subscriptiondomain.ErrInvalidTargetStatus
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subscription, err := s.repo.FindByIDForUpdate(ctx, tx, orgID, id)
		if err != nil {
			return err
		}
		if subscription == nil {
			return subscriptiondomain.ErrSubscriptionNotFound
		}

		if subscription.Status == targetStatus {
			return nil
		}

		if !isTransitionAllowed(subscription.Status, targetStatus) {
			return subscriptiondomain.ErrInvalidTransition
		}

		now := time.Now().UTC()
		switch targetStatus {
		case subscriptiondomain.SubscriptionStatusActive:
			if subscription.Status == subscriptiondomain.SubscriptionStatusDraft {
				if err := s.validateActivation(ctx, tx, subscription); err != nil {
					return err
				}
				if subscription.ActivatedAt == nil {
					subscription.ActivatedAt = &now
				}
			}
			if subscription.Status == subscriptiondomain.SubscriptionStatusPaused {
				subscription.ResumedAt = &now
			}
		case subscriptiondomain.SubscriptionStatusPaused:
			subscription.PausedAt = &now
		case subscriptiondomain.SubscriptionStatusCanceled:
			subscription.CanceledAt = &now
		case subscriptiondomain.SubscriptionStatusEnded:
			if err := s.validateEnd(ctx, tx, subscription); err != nil {
				return err
			}
			subscription.EndedAt = &now
		default:
			return subscriptiondomain.ErrInvalidTargetStatus
		}

		subscription.Status = targetStatus
		subscription.UpdatedAt = now

		return s.updateLifecycle(ctx, tx, subscription)
	})
}

func (s *Service) parseID(value string, invalidErr error) (snowflake.ID, error) {
	id, err := snowflake.ParseString(strings.TrimSpace(value))
	if err != nil || id == 0 {
		return 0, invalidErr
	}
	return id, nil
}

func (s *Service) orgIDFromContext(ctx context.Context) (snowflake.ID, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0, subscriptiondomain.ErrInvalidOrganization
	}
	return snowflake.ID(orgID), nil
}

func (s *Service) validateActivation(ctx context.Context, tx *gorm.DB, subscription *subscriptiondomain.Subscription) error {
	itemCount, err := s.countSubscriptionItems(ctx, tx, subscription.OrgID, subscription.ID)
	if err != nil {
		return err
	}
	if itemCount == 0 {
		return subscriptiondomain.ErrMissingSubscriptionItems
	}

	pricedCount, err := s.countSubscriptionItemsWithPrice(ctx, tx, subscription.OrgID, subscription.ID)
	if err != nil {
		return err
	}
	if pricedCount < itemCount {
		return subscriptiondomain.ErrMissingPricing
	}

	hasCustomer, err := s.hasCustomer(ctx, tx, subscription.OrgID, subscription.CustomerID)
	if err != nil {
		return err
	}
	if !hasCustomer {
		return subscriptiondomain.ErrMissingCustomer
	}

	return nil
}

func (s *Service) validateEnd(ctx context.Context, tx *gorm.DB, subscription *subscriptiondomain.Subscription) error {
	openCycles, err := s.countOpenBillingCycles(ctx, tx, subscription.OrgID, subscription.ID)
	if err != nil {
		return err
	}
	if openCycles > 0 {
		return subscriptiondomain.ErrBillingCyclesOpen
	}

	openInvoices, err := s.countUnfinalizedInvoices(ctx, tx, subscription.OrgID, subscription.ID)
	if err != nil {
		return err
	}
	if openInvoices > 0 {
		return subscriptiondomain.ErrInvoicesNotFinalized
	}

	return nil
}

func (s *Service) updateLifecycle(ctx context.Context, tx *gorm.DB, subscription *subscriptiondomain.Subscription) error {
	return tx.WithContext(ctx).Exec(
		`UPDATE subscriptions
		 SET status = ?, activated_at = ?, paused_at = ?, resumed_at = ?, canceled_at = ?, ended_at = ?, updated_at = ?
		 WHERE org_id = ? AND id = ?`,
		subscription.Status,
		subscription.ActivatedAt,
		subscription.PausedAt,
		subscription.ResumedAt,
		subscription.CanceledAt,
		subscription.EndedAt,
		subscription.UpdatedAt,
		subscription.OrgID,
		subscription.ID,
	).Error
}

func (s *Service) countSubscriptionItems(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1) FROM subscription_items WHERE org_id = ? AND subscription_id = ?`,
		orgID,
		subscriptionID,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) countSubscriptionItemsWithPrice(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM subscription_items si
		 JOIN prices p ON p.id = si.price_id AND p.org_id = si.org_id
		 WHERE si.org_id = ? AND si.subscription_id = ?`,
		orgID,
		subscriptionID,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) hasCustomer(ctx context.Context, tx *gorm.DB, orgID, customerID snowflake.ID) (bool, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1) FROM customers WHERE org_id = ? AND id = ?`,
		orgID,
		customerID,
	).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Service) countOpenBillingCycles(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ? AND status != ?`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusClosed,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) countUnfinalizedInvoices(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM invoices
		 WHERE org_id = ? AND subscription_id = ? AND status NOT IN (?, ?)`,
		orgID,
		subscriptionID,
		invoicedomain.InvoiceStatusFinalized,
		invoicedomain.InvoiceStatusVoid,
	).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func isValidStatus(status subscriptiondomain.SubscriptionStatus) bool {
	switch status {
	case subscriptiondomain.SubscriptionStatusDraft,
		subscriptiondomain.SubscriptionStatusActive,
		subscriptiondomain.SubscriptionStatusPaused,
		subscriptiondomain.SubscriptionStatusCanceled,
		subscriptiondomain.SubscriptionStatusEnded:
		return true
	default:
		return false
	}
}

func isTransitionAllowed(current, target subscriptiondomain.SubscriptionStatus) bool {
	switch current {
	case subscriptiondomain.SubscriptionStatusDraft:
		return target == subscriptiondomain.SubscriptionStatusActive
	case subscriptiondomain.SubscriptionStatusActive:
		return target == subscriptiondomain.SubscriptionStatusPaused || target == subscriptiondomain.SubscriptionStatusCanceled
	case subscriptiondomain.SubscriptionStatusPaused:
		return target == subscriptiondomain.SubscriptionStatusActive || target == subscriptiondomain.SubscriptionStatusCanceled
	case subscriptiondomain.SubscriptionStatusCanceled:
		return target == subscriptiondomain.SubscriptionStatusEnded
	default:
		return false
	}
}

func parseStatusFilter(value string) (*subscriptiondomain.SubscriptionStatus, error) {
	status := strings.TrimSpace(value)
	if status == "" {
		return nil, nil
	}

	status = strings.ToUpper(status)
	switch subscriptiondomain.SubscriptionStatus(status) {
	case subscriptiondomain.SubscriptionStatusDraft,
		subscriptiondomain.SubscriptionStatusActive,
		subscriptiondomain.SubscriptionStatusCanceled,
		subscriptiondomain.SubscriptionStatusPaused,
		subscriptiondomain.SubscriptionStatusEnded:
		parsed := subscriptiondomain.SubscriptionStatus(status)
		return &parsed, nil
	default:
		return nil, subscriptiondomain.ErrInvalidStatus
	}
}

func parseStatus(value string) (subscriptiondomain.SubscriptionStatus, error) {
	status := strings.TrimSpace(value)
	if status == "" {
		return subscriptiondomain.SubscriptionStatusActive, nil
	}

	switch subscriptiondomain.SubscriptionStatus(status) {
	case subscriptiondomain.SubscriptionStatusDraft,
		subscriptiondomain.SubscriptionStatusActive,
		subscriptiondomain.SubscriptionStatusCanceled,
		subscriptiondomain.SubscriptionStatusPaused,
		subscriptiondomain.SubscriptionStatusEnded:
		return subscriptiondomain.SubscriptionStatus(status), nil
	default:
		return "", subscriptiondomain.ErrInvalidStatus
	}
}

func parseCollectionMode(value string) (subscriptiondomain.SubscriptionCollectionMode, error) {
	mode := strings.TrimSpace(value)
	if mode == "" {
		return "", subscriptiondomain.ErrInvalidCollectionMode
	}

	switch subscriptiondomain.SubscriptionCollectionMode(mode) {
	case subscriptiondomain.SendInvoice,
		subscriptiondomain.ChargeAutomatically:
		return subscriptiondomain.SubscriptionCollectionMode(mode), nil
	default:
		return "", subscriptiondomain.ErrInvalidCollectionMode
	}
}

func (s *Service) buildSubscriptionItems(
	ctx context.Context,
	orgID snowflake.ID,
	subscriptionID snowflake.ID,
	items []subscriptiondomain.CreateSubscriptionItemRequest,
	now time.Time,
) ([]subscriptiondomain.SubscriptionItem, string, error) {
	priceCache := make(map[string]*pricedomain.Response, len(items))
	flatCount := 0
	subscriptionItems := make([]subscriptiondomain.SubscriptionItem, 0, len(items))
	billingCycleType := ""

	for _, item := range items {
		price, err := s.loadPrice(ctx, item.PriceID, priceCache)
		if err != nil {
			return nil, "", err
		}

		if err := validateSubscriptionPricingModel(price, &flatCount); err != nil {
			return nil, "", err
		}

		quantity := normalizeSubscriptionQuantity(item.Quantity)
		if err := validateSubscriptionBillingMode(price, quantity); err != nil {
			return nil, "", err
		}

		parsedPriceID, err := s.parseID(price.ID, subscriptiondomain.ErrInvalidPrice)
		if err != nil {
			return nil, "", err
		}

		cycleType, err := billingCycleTypeForInterval(price.BillingInterval)
		if err != nil {
			return nil, "", err
		}
		if billingCycleType == "" {
			billingCycleType = cycleType
		} else if billingCycleType != cycleType {
			return nil, "", subscriptiondomain.ErrInvalidBillingCycleType
		}

		meterID, meterCode, err := s.resolvePriceMeter(ctx, orgID, parsedPriceID)
		if err != nil {
			return nil, "", err
		}

		var priceCodePtr *string
		if price.Code != "" {
			code := price.Code
			priceCodePtr = &code
		}

		subscriptionItems = append(subscriptionItems, subscriptiondomain.SubscriptionItem{
			ID:               s.genID.Generate(),
			OrgID:            orgID,
			SubscriptionID:   subscriptionID,
			PriceID:          parsedPriceID,
			PriceCode:        priceCodePtr, // snapshot
			MeterID:          meterID,
			MeterCode:        meterCode, // snapshot
			Quantity:         quantity,
			BillingMode:      string(price.BillingMode), // snapshot
			BillingThreshold: price.BillingThreshold,    // snapshot
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}

	if billingCycleType == "" {
		return nil, "", subscriptiondomain.ErrInvalidBillingCycleType
	}

	return subscriptionItems, billingCycleType, nil
}

func (s *Service) loadPrice(
	ctx context.Context,
	priceID string,
	cache map[string]*pricedomain.Response,
) (*pricedomain.Response, error) {
	trimmed := strings.TrimSpace(priceID)
	if trimmed == "" {
		return nil, subscriptiondomain.ErrInvalidPrice
	}

	if cached, ok := cache[trimmed]; ok {
		return cached, nil
	}

	loaded, err := s.pricesvc.Get(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	if !loaded.Active || loaded.RetiredAt != nil {
		return nil, subscriptiondomain.ErrInvalidPrice
	}

	cache[trimmed] = loaded
	return loaded, nil
}

func validateSubscriptionPricingModel(price *pricedomain.Response, flatCount *int) error {
	switch price.PricingModel {
	case pricedomain.Flat:
		*flatCount++
		if *flatCount > 1 {
			return subscriptiondomain.ErrMultipleFlatPrices
		}
		return nil
	case pricedomain.PerUnit,
		pricedomain.TieredVolume,
		pricedomain.TieredGraduated:
		return nil
	default:
		return pricedomain.ErrUnsupportedPricingModel
	}
}

func normalizeSubscriptionQuantity(quantity int8) int8 {
	if quantity <= 0 {
		return 1
	}
	return quantity
}

func validateSubscriptionBillingMode(price *pricedomain.Response, quantity int8) error {
	switch price.BillingMode {
	case pricedomain.Licensed:
		if quantity < 1 {
			return pricedomain.ErrInvalidPricingModel
		}
		return nil
	case pricedomain.Metered:
		return nil
	default:
		return pricedomain.ErrInvalidBillingMode
	}
}

func (s *Service) toCreateResponse(subscription *subscriptiondomain.Subscription, items []subscriptiondomain.SubscriptionItem) subscriptiondomain.CreateSubscriptionResponse {
	respItems := make([]subscriptiondomain.CreateSubscriptionItemResponse, 0, len(items))
	for _, item := range items {
		var meterID *string
		if item.MeterID != nil {
			value := item.MeterID.String()
			meterID = &value
		}
		var meterCode *string
		if item.MeterCode != nil && strings.TrimSpace(*item.MeterCode) != "" {
			value := strings.TrimSpace(*item.MeterCode)
			meterCode = &value
		}

		respItems = append(respItems, subscriptiondomain.CreateSubscriptionItemResponse{
			ID:                item.ID.String(),
			PriceID:           item.PriceID.String(),
			PriceCode:         item.PriceCode,
			MeterID:           meterID,
			MeterCode:         meterCode,
			Quantity:          item.Quantity,
			BillingMode:       item.BillingMode,
			UsageBehavior:     item.UsageBehavior,
			BillingThreshold:  item.BillingThreshold,
			ProrationBehavior: item.ProrationBehavior,
		})
	}

	var metadata map[string]any
	if subscription.Metadata != nil {
		metadata = map[string]any(subscription.Metadata)
	}

	return subscriptiondomain.CreateSubscriptionResponse{
		ID:             subscription.ID.String(),
		OrganizationID: subscription.OrgID.String(),
		CustomerID:     subscription.CustomerID.String(),
		Status:         subscription.Status,
		CollectionMode: subscription.CollectionMode,
		StartAt:        subscription.StartAt,
		Items:          respItems,
		Metadata:       metadata,
	}
}

func (s *Service) resolvePriceMeter(ctx context.Context, orgID, priceID snowflake.ID) (*snowflake.ID, *string, error) {
	var row struct {
		MeterID   snowflake.ID `gorm:"column:meter_id"`
		MeterCode string       `gorm:"column:code"`
	}
	if err := s.db.WithContext(ctx).Raw(
		`SELECT m.id AS meter_id, m.code
		 FROM price_amounts pa
		 JOIN meters m ON m.id = pa.meter_id
		 WHERE pa.org_id = ? AND pa.price_id = ? AND pa.meter_id IS NOT NULL
		 ORDER BY pa.meter_id DESC
		 LIMIT 1`,
		orgID,
		priceID,
	).Scan(&row).Error; err != nil {
		return nil, nil, err
	}
	if row.MeterID == 0 {
		return nil, nil, subscriptiondomain.ErrInvalidMeterID
	}
	meterCode := strings.TrimSpace(row.MeterCode)
	if meterCode == "" {
		return nil, nil, subscriptiondomain.ErrInvalidMeterCode
	}
	meterID := row.MeterID
	return &meterID, &meterCode, nil
}

func billingCycleTypeForInterval(interval pricedomain.BillingInterval) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(string(interval))) {
	case string(pricedomain.Day):
		return "daily", nil
	case string(pricedomain.Week):
		return "weekly", nil
	case string(pricedomain.Month):
		return "monthly", nil
	default:
		return "", subscriptiondomain.ErrInvalidBillingCycleType
	}
}
