package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	authscope "github.com/smallbiznis/valora/internal/auth/scope"
	"github.com/smallbiznis/valora/internal/orgcontext"
)

type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorAPIKey ActorType = "api_key"
	ActorSystem ActorType = "system"
)

type Actor struct {
	Type   ActorType
	OrgID  snowflake.ID
	ID     string
	Scopes []string
}

func (s *Server) authorizeOrgAction(object string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.authorizeOrgActionWithContext(c, object, action); err != nil {
			AbortWithError(c, err)
			return
		}
		c.Next()
	}
}

func (s *Server) authorizeOrgActionWithContext(c *gin.Context, object string, action string) error {
	actor, ok := s.actorFromContext(c)
	if !ok {
		return ErrUnauthorized
	}

	orgID := actor.OrgID
	if orgID == 0 {
		var err error
		orgID, err = s.orgIDFromRequest(c)
		if err != nil {
			return err
		}
	}

	switch actor.Type {
	case ActorAPIKey:
		requiredScope := authscope.FromAuthz(object, action)
		if !authscope.Has(actor.Scopes, requiredScope) {
			return ErrForbidden
		}
		return nil
	case ActorUser:
		if s.authzSvc == nil {
			return ErrForbidden
		}
		return s.authorizeForOrg(c, actor.subject(), orgID, object, action)
	case ActorSystem:
		if !allowSystemAction(object, action) {
			return ErrForbidden
		}
		return nil
	default:
		return ErrUnauthorized
	}
}

func (s *Server) authorizeForOrg(c *gin.Context, actor string, orgID snowflake.ID, object string, action string) error {
	return s.authzSvc.Authorize(c.Request.Context(), actor, orgID.String(), strings.TrimSpace(object), strings.TrimSpace(action))
}

func (s *Server) actorFromContext(c *gin.Context) (Actor, bool) {
	if c == nil {
		return Actor{}, false
	}

	ctx := c.Request.Context()
	orgID := orgIDFromContext(ctx)

	if authType, ok := ctx.Value(contextAuthTypeKey).(string); ok {
		normalized := strings.TrimSpace(authType)
		switch normalized {
		case string(ActorAPIKey):
			apiKeyID, ok := apiKeyIDFromContext(ctx)
			if !ok {
				return Actor{}, false
			}
			return Actor{
				Type:   ActorAPIKey,
				OrgID:  orgID,
				ID:     apiKeyID.String(),
				Scopes: apiKeyScopesFromContext(ctx),
			}, true
		case string(ActorSystem):
			return Actor{
				Type:  ActorSystem,
				OrgID: orgID,
				ID:    "system",
			}, true
		}
	}

	userID, ok := s.userIDFromSession(c)
	if !ok {
		return Actor{}, false
	}
	return Actor{Type: ActorUser, OrgID: orgID, ID: userID.String()}, true
}

func orgIDFromContext(ctx context.Context) snowflake.ID {
	if ctx == nil {
		return 0
	}
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return 0
	}
	return snowflake.ID(orgID)
}

func (a Actor) subject() string {
	switch a.Type {
	case ActorUser:
		return fmt.Sprintf("user:%s", a.ID)
	case ActorAPIKey:
		return fmt.Sprintf("api_key:%s", a.ID)
	case ActorSystem:
		return "system"
	default:
		return ""
	}
}

func allowSystemAction(object string, action string) bool {
	key := strings.ToLower(strings.TrimSpace(object)) + ":" + strings.ToLower(strings.TrimSpace(action))
	switch key {
	default:
		return false
	}
}

func apiKeyIDFromContext(ctx context.Context) (snowflake.ID, bool) {
	if ctx == nil {
		return 0, false
	}
	raw := ctx.Value(contextAPIKeyIDKey)
	switch value := raw.(type) {
	case int64:
		if value == 0 {
			return 0, false
		}
		return snowflake.ID(value), true
	case snowflake.ID:
		if value == 0 {
			return 0, false
		}
		return value, true
	case string:
		parsed, err := snowflake.ParseString(strings.TrimSpace(value))
		if err != nil || parsed == 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func apiKeyScopesFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	value := ctx.Value(contextAPIKeyScopesKey)
	scopes, ok := value.([]string)
	if !ok {
		return nil
	}
	return scopes
}
