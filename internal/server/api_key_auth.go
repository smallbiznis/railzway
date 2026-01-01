package server

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	apikeydomain "github.com/smallbiznis/valora/internal/apikey/domain"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	auditcontext "github.com/smallbiznis/valora/internal/auditcontext"
	"github.com/smallbiznis/valora/internal/orgcontext"
)

const (
	contextAuthTypeKey     = "auth_type"
	contextOrgIDKey        = "org_id"
	contextAPIKeyIDKey     = "api_key_id"
	contextAPIKeyScopesKey = "api_key_scopes"
)

// APIKeyRequired authenticates requests using an API key only.
// Organization identity is derived solely from the api_keys table.
func (s *Server) APIKeyRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if requestHasOrgID(c) {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		parts := strings.Fields(header)
		if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		hash := apikeydomain.HashAPIKey(parts[1])
		now := time.Now().UTC()

		var record struct {
			ID      snowflake.ID   `gorm:"column:id"`
			OrgID   snowflake.ID   `gorm:"column:org_id"`
			KeyHash string         `gorm:"column:key_hash"`
			Scopes  pq.StringArray `gorm:"column:scopes"`
		}

		if err := s.db.WithContext(c.Request.Context()).Raw(
			`SELECT id, org_id, key_hash, scopes
			 FROM api_keys
			 WHERE key_hash = ?
			   AND is_active = true
			   AND (expires_at IS NULL OR expires_at > ?)
			 LIMIT 1`,
			hash,
			now,
		).Scan(&record).Error; err != nil {
			AbortWithError(c, err)
			return
		}

		if record.ID == 0 || subtle.ConstantTimeCompare([]byte(record.KeyHash), []byte(hash)) != 1 {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		ctx := c.Request.Context()
		scopes := make([]string, 0, len(record.Scopes))
		scopes = append(scopes, record.Scopes...)
		ctx = context.WithValue(ctx, contextAuthTypeKey, string(ActorAPIKey))
		ctx = context.WithValue(ctx, contextOrgIDKey, int64(record.OrgID))
		ctx = context.WithValue(ctx, contextAPIKeyIDKey, int64(record.ID))
		ctx = context.WithValue(ctx, contextAPIKeyScopesKey, scopes)
		ctx = orgcontext.WithOrgID(ctx, int64(record.OrgID))
		ctx = auditcontext.WithActor(ctx, string(auditdomain.ActorTypeAPIKey), record.ID.String())

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func requestHasOrgID(c *gin.Context) bool {
	if strings.TrimSpace(c.GetHeader(HeaderOrg)) != "" {
		return true
	}
	if value, ok := c.GetQuery("org_id"); ok && strings.TrimSpace(value) != "" {
		return true
	}
	if value, ok := c.GetQuery("orgId"); ok && strings.TrimSpace(value) != "" {
		return true
	}
	return false
}
