package authorization

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

//go:embed model.conf
var modelText string

const (
	ObjectSubscription      = "subscription"
	ObjectBillingCycle      = "billing_cycle"
	ObjectInvoice           = "invoice"
	ObjectBillingDashboard  = "billing_dashboard"
	ObjectBillingOperations = "billing_operations"
	ObjectBillingOverview   = "billing_overview"
	ObjectAPIKey            = "api_key"
	ObjectAuditLog          = "audit_log"
	ObjectPaymentProvider   = "payment_provider"
)

const (
	ActionSubscriptionActivate = "subscription.activate"
	ActionSubscriptionPause    = "subscription.pause"
	ActionSubscriptionResume   = "subscription.resume"
	ActionSubscriptionCancel   = "subscription.cancel"
	ActionSubscriptionEnd      = "subscription.end"

	ActionBillingCycleOpen         = "billing_cycle.open"
	ActionBillingCycleStartClosing = "billing_cycle.start_closing"
	ActionBillingCycleClose        = "billing_cycle.close"
	ActionBillingCycleRate         = "billing_cycle.rate"

	ActionInvoiceGenerate = "invoice.generate"
	ActionInvoiceFinalize = "invoice.finalize"
	ActionInvoiceVoid     = "invoice.void"

	ActionBillingDashboardView  = "billing_dashboard.view"
	ActionBillingOperationsView = "billing_operations.view"
	ActionBillingOperationsAct  = "billing_operations.act"
	ActionBillingOverviewView   = "billing_overview.view"

	ActionAPIKeyView   = "api_key.view"
	ActionAPIKeyCreate = "api_key.create"
	ActionAPIKeyRotate = "api_key.rotate"
	ActionAPIKeyRevoke = "api_key.revoke"

	ActionAuditLogView = "audit_log.view"

	ActionPaymentProviderManage = "payment_provider.manage"
)

type Params struct {
	fx.In

	DB       *gorm.DB
	Log      *zap.Logger
	Enforcer *casbin.SyncedEnforcer
	AuditSvc auditdomain.Service `optional:"true"`
}

type ServiceImpl struct {
	db       *gorm.DB
	log      *zap.Logger
	enforcer *casbin.SyncedEnforcer
	auditSvc auditdomain.Service
}

func NewEnforcer(db *gorm.DB) (*casbin.SyncedEnforcer, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}
	enforcer.EnableAutoSave(true)
	enforcer.EnableAutoBuildRoleLinks(true)
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, err
	}
	if err := seedPolicies(enforcer); err != nil {
		return nil, err
	}
	enforcer.BuildRoleLinks()
	return enforcer, nil
}

func NewService(p Params) Service {
	return &ServiceImpl{
		db:       p.DB,
		log:      p.Log.Named("authorization.service"),
		enforcer: p.Enforcer,
		auditSvc: p.AuditSvc,
	}
}

func (s *ServiceImpl) Authorize(ctx context.Context, actor string, orgID string, object string, action string) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return ErrInvalidActor
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return ErrInvalidOrganization
	}
	object = strings.TrimSpace(object)
	if object == "" {
		return ErrInvalidObject
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return ErrInvalidAction
	}

	subject, roleName, actorType, actorID, err := s.resolveActor(ctx, actor, orgID)
	if err != nil {
		s.auditDenied(ctx, actorType, actorID, orgID, object, action)
		return err
	}

	domain := fmt.Sprintf("org:%s", orgID)
	if err := s.ensureGrouping(subject, roleName, domain); err != nil {
		return err
	}

	allowed, err := s.enforcer.Enforce(subject, domain, object, action)
	if err != nil {
		return err
	}
	if !allowed {
		s.auditDenied(ctx, actorType, actorID, orgID, object, action)
		return ErrForbidden
	}

	if shouldAuditGrant(action) {
		s.auditGranted(ctx, actorType, actorID, orgID, object, action)
	}
	return nil
}

func (s *ServiceImpl) resolveActor(ctx context.Context, actor string, orgID string) (string, string, string, *string, error) {
	if actor == "system" {
		roleName := "role:system"
		return actor, roleName, "system", nil, nil
	}
	if strings.HasPrefix(actor, "user:") {
		userIDRaw := strings.TrimPrefix(actor, "user:")
		userID, err := snowflake.ParseString(userIDRaw)
		if err != nil || userID == 0 {
			return "", "", "", nil, ErrInvalidActor
		}
		parsedOrgID, err := snowflake.ParseString(orgID)
		userIDStr := userID.String()
		if err != nil || parsedOrgID == 0 {
			return actor, "", "user", &userIDStr, ErrInvalidOrganization
		}
		role, err := s.roleForUser(ctx, parsedOrgID, userID)
		if err != nil {
			return actor, "", "user", &userIDStr, err
		}
		roleName := fmt.Sprintf("role:%s", strings.ToLower(role))
		return actor, roleName, "user", &userIDStr, nil
	}
	return "", "", "", nil, ErrInvalidActor
}

func (s *ServiceImpl) roleForUser(ctx context.Context, orgID snowflake.ID, userID snowflake.ID) (string, error) {
	var row struct {
		Role string `gorm:"column:role"`
	}
	if err := s.db.WithContext(ctx).Raw(
		`SELECT role
		 FROM organization_members
		 WHERE org_id = ? AND user_id = ?
		 LIMIT 1`,
		orgID,
		userID,
	).Scan(&row).Error; err != nil {
		return "", err
	}

	role := strings.TrimSpace(row.Role)
	if role == "" {
		return "", ErrForbidden
	}
	return role, nil
}

func (s *ServiceImpl) ensureGrouping(subject string, roleName string, domain string) error {
	existing, err := s.enforcer.GetFilteredGroupingPolicy(0, subject, "", domain)
	if err != nil {
		return err
	}
	for _, rule := range existing {
		if len(rule) < 2 {
			continue
		}
		if rule[1] != roleName {
			params := make([]interface{}, 0, len(rule))
			for _, value := range rule {
				params = append(params, value)
			}
			_, _ = s.enforcer.RemoveGroupingPolicy(params...)
		}
	}

	has, err := s.enforcer.HasGroupingPolicy(subject, roleName, domain)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = s.enforcer.AddGroupingPolicy(subject, roleName, domain)
	return err
}

func (s *ServiceImpl) auditDenied(ctx context.Context, actorType string, actorID *string, orgID string, object string, action string) {
	if s.auditSvc == nil {
		return
	}
	parsedOrgID, err := snowflake.ParseString(orgID)
	if err != nil || parsedOrgID == 0 {
		return
	}
	targetID := "capability"
	_ = s.auditSvc.AuditLog(ctx, &parsedOrgID, actorType, actorID, "authorization.denied", "authorization", &targetID, map[string]any{
		"object":  object,
		"action":  action,
		"actor":   actorType,
		"org_id":  orgID,
		"subject": actorSubject(actorType, actorID),
	})
}

func (s *ServiceImpl) auditGranted(ctx context.Context, actorType string, actorID *string, orgID string, object string, action string) {
	if s.auditSvc == nil {
		return
	}
	parsedOrgID, err := snowflake.ParseString(orgID)
	if err != nil || parsedOrgID == 0 {
		return
	}
	targetID := "capability"
	_ = s.auditSvc.AuditLog(ctx, &parsedOrgID, actorType, actorID, "authorization.granted", "authorization", &targetID, map[string]any{
		"object":  object,
		"action":  action,
		"actor":   actorType,
		"org_id":  orgID,
		"subject": actorSubject(actorType, actorID),
	})
}

func actorSubject(actorType string, actorID *string) string {
	switch actorType {
	case "system":
		return "system"
	case "user":
		if actorID != nil && strings.TrimSpace(*actorID) != "" {
			return fmt.Sprintf("user:%s", strings.TrimSpace(*actorID))
		}
	}
	return ""
}

func shouldAuditGrant(action string) bool {
	switch action {
	case ActionAPIKeyRotate, ActionAPIKeyRevoke, ActionInvoiceVoid:
		return true
	default:
		return false
	}
}

func seedPolicies(enforcer *casbin.SyncedEnforcer) error {
	policies := [][]string{
		{"role:member", ObjectSubscription, "view"},
		{"role:member", ObjectInvoice, "view"},

		{"role:admin", ObjectSubscription, ActionSubscriptionActivate},
		{"role:admin", ObjectSubscription, ActionSubscriptionPause},
		{"role:admin", ObjectSubscription, ActionSubscriptionResume},
		{"role:admin", ObjectInvoice, ActionInvoiceFinalize},
		{"role:admin", ObjectBillingDashboard, ActionBillingDashboardView},
		{"role:admin", ObjectBillingOperations, ActionBillingOperationsView},
		{"role:admin", ObjectBillingOperations, ActionBillingOperationsAct},
		{"role:admin", ObjectBillingOverview, ActionBillingOverviewView},
		{"role:admin", ObjectAPIKey, ActionAPIKeyCreate},
		{"role:admin", ObjectAPIKey, ActionAPIKeyRotate},
		{"role:admin", ObjectAPIKey, ActionAPIKeyView},
		{"role:admin", ObjectAuditLog, ActionAuditLogView},
		{"role:admin", ObjectPaymentProvider, ActionPaymentProviderManage},

		{"role:owner", ObjectSubscription, ActionSubscriptionActivate},
		{"role:owner", ObjectSubscription, ActionSubscriptionPause},
		{"role:owner", ObjectSubscription, ActionSubscriptionResume},
		{"role:owner", ObjectSubscription, ActionSubscriptionCancel},
		{"role:owner", ObjectInvoice, ActionInvoiceFinalize},
		{"role:owner", ObjectInvoice, ActionInvoiceVoid},
		{"role:owner", ObjectBillingDashboard, ActionBillingDashboardView},
		{"role:owner", ObjectBillingOperations, ActionBillingOperationsView},
		{"role:owner", ObjectBillingOperations, ActionBillingOperationsAct},
		{"role:owner", ObjectBillingOverview, ActionBillingOverviewView},
		{"role:owner", ObjectAPIKey, ActionAPIKeyView},
		{"role:owner", ObjectAPIKey, ActionAPIKeyCreate},
		{"role:owner", ObjectAPIKey, ActionAPIKeyRotate},
		{"role:owner", ObjectAPIKey, ActionAPIKeyRevoke},
		{"role:owner", ObjectAuditLog, ActionAuditLogView},
		{"role:owner", ObjectPaymentProvider, ActionPaymentProviderManage},
		
		{"role:finops", ObjectBillingOperations, ActionBillingOperationsView},
		{"role:finops", ObjectBillingOperations, ActionBillingOperationsAct},
		{"role:finops", ObjectBillingDashboard, ActionBillingDashboardView},
		{"role:finops", ObjectBillingOverview, ActionBillingOverviewView},
		{"role:finops", ObjectInvoice, "view"},

		{"role:system", ObjectSubscription, ActionSubscriptionEnd},
		{"role:system", ObjectBillingCycle, ActionBillingCycleOpen},
		{"role:system", ObjectBillingCycle, ActionBillingCycleStartClosing},
		{"role:system", ObjectBillingCycle, ActionBillingCycleRate},
		{"role:system", ObjectBillingCycle, ActionBillingCycleClose},
		{"role:system", ObjectInvoice, ActionInvoiceGenerate},
		{"role:system", ObjectInvoice, ActionInvoiceFinalize},
	}

	for _, policy := range policies {
		if len(policy) < 3 {
			continue
		}
		if _, err := enforcer.AddPolicy(policy); err != nil {
			return err
		}
	}
	return nil
}
