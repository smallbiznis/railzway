package scope

import (
	"errors"
	"strings"

	"github.com/smallbiznis/valora/internal/authorization"
)

type Scope string

var ErrInvalidScope = errors.New("invalid_scope")

const (
	ScopeSubscriptionActivate Scope = "subscription:activate"
	ScopeSubscriptionPause    Scope = "subscription:pause"
	ScopeSubscriptionResume   Scope = "subscription:resume"
	ScopeSubscriptionCancel   Scope = "subscription:cancel"
	ScopeSubscriptionEnd      Scope = "subscription:end"

	ScopeBillingCycleOpen         Scope = "billing_cycle:open"
	ScopeBillingCycleStartClosing Scope = "billing_cycle:start_closing"
	ScopeBillingCycleClose        Scope = "billing_cycle:close"
	ScopeBillingCycleRate         Scope = "billing_cycle:rate"

	ScopeInvoiceGenerate Scope = "invoice:generate"
	ScopeInvoiceFinalize Scope = "invoice:finalize"
	ScopeInvoiceVoid     Scope = "invoice:void"

	ScopeAPIKeyView   Scope = "api_key:view"
	ScopeAPIKeyCreate Scope = "api_key:create"
	ScopeAPIKeyRotate Scope = "api_key:rotate"
	ScopeAPIKeyRevoke Scope = "api_key:revoke"

	ScopeAuditLogView Scope = "audit_log:view"

	ScopeUsageIngest Scope = "usage:ingest"
	ScopeUsageWrite  Scope = "usage:write"
)

type authzKey struct {
	object string
	action string
}

var authzScopeMap = map[authzKey]Scope{
	{normalize(authorization.ObjectSubscription), normalize(authorization.ActionSubscriptionActivate)}: ScopeSubscriptionActivate,
	{normalize(authorization.ObjectSubscription), normalize(authorization.ActionSubscriptionPause)}:    ScopeSubscriptionPause,
	{normalize(authorization.ObjectSubscription), normalize(authorization.ActionSubscriptionResume)}:   ScopeSubscriptionResume,
	{normalize(authorization.ObjectSubscription), normalize(authorization.ActionSubscriptionCancel)}:   ScopeSubscriptionCancel,
	{normalize(authorization.ObjectSubscription), normalize(authorization.ActionSubscriptionEnd)}:      ScopeSubscriptionEnd,

	{normalize(authorization.ObjectBillingCycle), normalize(authorization.ActionBillingCycleOpen)}:         ScopeBillingCycleOpen,
	{normalize(authorization.ObjectBillingCycle), normalize(authorization.ActionBillingCycleStartClosing)}: ScopeBillingCycleStartClosing,
	{normalize(authorization.ObjectBillingCycle), normalize(authorization.ActionBillingCycleClose)}:        ScopeBillingCycleClose,
	{normalize(authorization.ObjectBillingCycle), normalize(authorization.ActionBillingCycleRate)}:         ScopeBillingCycleRate,

	{normalize(authorization.ObjectInvoice), normalize(authorization.ActionInvoiceGenerate)}: ScopeInvoiceGenerate,
	{normalize(authorization.ObjectInvoice), normalize(authorization.ActionInvoiceFinalize)}: ScopeInvoiceFinalize,
	{normalize(authorization.ObjectInvoice), normalize(authorization.ActionInvoiceVoid)}:     ScopeInvoiceVoid,

	{normalize(authorization.ObjectAPIKey), normalize(authorization.ActionAPIKeyView)}:   ScopeAPIKeyView,
	{normalize(authorization.ObjectAPIKey), normalize(authorization.ActionAPIKeyCreate)}: ScopeAPIKeyCreate,
	{normalize(authorization.ObjectAPIKey), normalize(authorization.ActionAPIKeyRotate)}: ScopeAPIKeyRotate,
	{normalize(authorization.ObjectAPIKey), normalize(authorization.ActionAPIKeyRevoke)}: ScopeAPIKeyRevoke,

	{normalize(authorization.ObjectAuditLog), normalize(authorization.ActionAuditLogView)}: ScopeAuditLogView,
}

var allScopes = []Scope{
	ScopeSubscriptionActivate,
	ScopeSubscriptionPause,
	ScopeSubscriptionResume,
	ScopeSubscriptionCancel,
	ScopeSubscriptionEnd,
	ScopeBillingCycleOpen,
	ScopeBillingCycleStartClosing,
	ScopeBillingCycleClose,
	ScopeBillingCycleRate,
	ScopeInvoiceGenerate,
	ScopeInvoiceFinalize,
	ScopeInvoiceVoid,
	ScopeAPIKeyView,
	ScopeAPIKeyCreate,
	ScopeAPIKeyRotate,
	ScopeAPIKeyRevoke,
	ScopeAuditLogView,
	ScopeUsageIngest,
	ScopeUsageWrite,
}

var validScopes = func() map[string]struct{} {
	lookup := make(map[string]struct{}, len(allScopes))
	for _, scope := range allScopes {
		lookup[normalize(string(scope))] = struct{}{}
	}
	return lookup
}()

func All() []string {
	values := make([]string, len(allScopes))
	for i, scope := range allScopes {
		values[i] = string(scope)
	}
	return values
}

func FromAuthz(object string, action string) Scope {
	key := authzKey{object: normalize(object), action: normalize(action)}
	if scope, ok := authzScopeMap[key]; ok {
		return scope
	}
	return ""
}

func Has(scopes []string, required Scope) bool {
	requiredScope := normalize(string(required))
	if requiredScope == "" {
		return false
	}

	requiredObject := strings.SplitN(requiredScope, ":", 2)[0]

	for _, scope := range scopes {
		normalized := normalize(scope)
		if normalized == "" {
			continue
		}
		if normalized == "*" {
			return true
		}
		if normalized == requiredScope {
			return true
		}
		if requiredObject != "" && (normalized == requiredObject+":*" || normalized == requiredObject+".*") {
			return true
		}
	}
	return false
}

func Validate(scopes []string) error {
	normalized := Normalize(scopes)
	for _, scope := range normalized {
		if !IsValid(scope) {
			return ErrInvalidScope
		}
	}
	return nil
}

func Normalize(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(scopes))
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		value := normalize(scope)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func IsValid(scope string) bool {
	_, ok := validScopes[normalize(scope)]
	return ok
}

func normalize(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.ReplaceAll(normalized, ".", ":")
}
