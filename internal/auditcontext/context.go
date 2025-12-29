package auditcontext

import "context"

type contextKey string

const (
	requestIDKey      contextKey = "audit_request_id"
	actorTypeKey      contextKey = "audit_actor_type"
	actorIDKey        contextKey = "audit_actor_id"
	ipAddressKey      contextKey = "audit_ip_address"
	userAgentKey      contextKey = "audit_user_agent"
	subscriptionIDKey contextKey = "audit_subscription_id"
	billingCycleIDKey contextKey = "audit_billing_cycle_id"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func WithActor(ctx context.Context, actorType, actorID string) context.Context {
	if actorType != "" {
		ctx = context.WithValue(ctx, actorTypeKey, actorType)
	}
	if actorID != "" {
		ctx = context.WithValue(ctx, actorIDKey, actorID)
	}
	return ctx
}

func ActorFromContext(ctx context.Context) (string, string) {
	actorType, _ := ctx.Value(actorTypeKey).(string)
	actorID, _ := ctx.Value(actorIDKey).(string)
	return actorType, actorID
}

func WithIPAddress(ctx context.Context, ipAddress string) context.Context {
	if ipAddress == "" {
		return ctx
	}
	return context.WithValue(ctx, ipAddressKey, ipAddress)
}

func IPAddressFromContext(ctx context.Context) string {
	value, _ := ctx.Value(ipAddressKey).(string)
	return value
}

func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	if userAgent == "" {
		return ctx
	}
	return context.WithValue(ctx, userAgentKey, userAgent)
}

func UserAgentFromContext(ctx context.Context) string {
	value, _ := ctx.Value(userAgentKey).(string)
	return value
}

func WithSubscriptionID(ctx context.Context, subscriptionID string) context.Context {
	if subscriptionID == "" {
		return ctx
	}
	return context.WithValue(ctx, subscriptionIDKey, subscriptionID)
}

func SubscriptionIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(subscriptionIDKey).(string)
	return value
}

func WithBillingCycleID(ctx context.Context, billingCycleID string) context.Context {
	if billingCycleID == "" {
		return ctx
	}
	return context.WithValue(ctx, billingCycleIDKey, billingCycleID)
}

func BillingCycleIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(billingCycleIDKey).(string)
	return value
}
