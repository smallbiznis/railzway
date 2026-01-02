package context

import "context"

type contextKey string

const (
	requestIDKey contextKey = "observability_request_id"
	orgIDKey     contextKey = "observability_org_id"
	actorTypeKey contextKey = "observability_actor_type"
	actorIDKey   contextKey = "observability_actor_id"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil || requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func WithOrgID(ctx context.Context, orgID string) context.Context {
	if ctx == nil || orgID == "" {
		return ctx
	}
	return context.WithValue(ctx, orgIDKey, orgID)
}

func OrgIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(orgIDKey).(string)
	return value
}

func WithActor(ctx context.Context, actorType, actorID string) context.Context {
	if ctx == nil {
		return ctx
	}
	if actorType != "" {
		ctx = context.WithValue(ctx, actorTypeKey, actorType)
	}
	if actorID != "" {
		ctx = context.WithValue(ctx, actorIDKey, actorID)
	}
	return ctx
}

func ActorFromContext(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	actorType, _ := ctx.Value(actorTypeKey).(string)
	actorID, _ := ctx.Value(actorIDKey).(string)
	return actorType, actorID
}
