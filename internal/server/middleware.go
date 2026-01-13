package server

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	auditcontext "github.com/smallbiznis/railzway/internal/auditcontext"
	authdomain "github.com/smallbiznis/railzway/internal/auth/domain"
	obscontext "github.com/smallbiznis/railzway/internal/observability/context"
	"github.com/smallbiznis/railzway/internal/orgcontext"
)

const (
	HeaderOrg = "X-Org-Id"
)

const (
	contextUserIDKey  = "user_id"
	contextSessionKey = "session"
)

func (s *Server) serveIndex(c *gin.Context) {
	c.File(filepath.Join(s.cfg.StaticDir, "index.html"))
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if requestID == "" {
			requestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
		}
		if requestID == "" {
			requestID = uuid.NewString()
		}

		c.Set("request_id", requestID)
		c.Header("X-Request-Id", requestID)

		ctx := auditcontext.WithRequestID(c.Request.Context(), requestID)
		ctx = auditcontext.WithIPAddress(ctx, c.ClientIP())
		ctx = auditcontext.WithUserAgent(ctx, c.Request.UserAgent())
		ctx = obscontext.WithRequestID(ctx, requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *Server) redirectIfLoggedIn() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := s.sessions.ReadToken(c)
		if !ok {
			c.Next()
			return
		}

		if _, err := s.authsvc.Authenticate(c.Request.Context(), sid); err != nil {
			c.Next()
			return
		}

		c.Redirect(302, "/orgs")
		c.Abort()
	}
}

func (s *Server) WebAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := s.sessions.ReadToken(c)
		if !ok {
			c.Redirect(302, "/login")
			c.Abort()
			return
		}

		session, err := s.authsvc.Authenticate(c.Request.Context(), sid)
		if err != nil {
			c.Redirect(302, "/login")
			c.Abort()
			return
		}

		c.Set(contextUserIDKey, session.UserID.String())
		c.Set(contextSessionKey, session)
		ctx := auditcontext.WithActor(c.Request.Context(), string(auditdomain.ActorTypeUser), session.UserID.String())
		ctx = obscontext.WithActor(ctx, string(auditdomain.ActorTypeUser), session.UserID.String())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *Server) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, ok := s.sessions.ReadToken(c)
		if !ok {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		session, err := s.authsvc.Authenticate(c.Request.Context(), sid)
		if err != nil {
			AbortWithError(c, err)
			return
		}

		c.Set(contextUserIDKey, session.UserID.String())
		c.Set(contextSessionKey, session)
		ctx := auditcontext.WithActor(c.Request.Context(), string(auditdomain.ActorTypeUser), session.UserID.String())
		ctx = obscontext.WithActor(ctx, string(auditdomain.ActorTypeUser), session.UserID.String())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *Server) OrgContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := s.sessionFromContext(c)
		if !ok {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		headerOrg := strings.TrimSpace(c.GetHeader(HeaderOrg))
		var resolvedOrgID int64
		if headerOrg != "" {
			parsed, err := snowflake.ParseString(headerOrg)
			if err != nil {
				AbortWithError(c, newValidationError("org_id", "invalid_org_id", "invalid org id"))
				return
			}
			resolvedOrgID = int64(parsed)
		} else if session.ActiveOrgID != nil {
			resolvedOrgID = *session.ActiveOrgID
		} else {
			AbortWithError(c, ErrOrgRequired)
			return
		}

		orgIDs := session.OrgIDs
		if !containsOrgID(orgIDs, resolvedOrgID) {
			freshOrgIDs, err := s.loadUserOrgIDs(c.Request.Context(), session.UserID)
			if err != nil {
				AbortWithError(c, err)
				return
			}
			orgIDs = freshOrgIDs
		}

		if !containsOrgID(orgIDs, resolvedOrgID) {
			AbortWithError(c, ErrForbidden)
			return
		}

		if headerOrg != "" {
			activeOrgID := resolvedOrgID
			if err := s.authsvc.UpdateSessionOrgContext(c.Request.Context(), session.ID, &activeOrgID, orgIDs); err != nil {
				AbortWithError(c, err)
				return
			}
			session.ActiveOrgID = &activeOrgID
			session.OrgIDs = orgIDs
		} else if len(session.OrgIDs) == 0 && len(orgIDs) > 0 {
			if err := s.authsvc.UpdateSessionOrgContext(c.Request.Context(), session.ID, session.ActiveOrgID, orgIDs); err != nil {
				AbortWithError(c, err)
				return
			}
			session.OrgIDs = orgIDs
		}

		ctx := orgcontext.WithOrgID(c.Request.Context(), resolvedOrgID)
		ctx = obscontext.WithOrgID(ctx, strconv.FormatInt(resolvedOrgID, 10))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *Server) RequireRole(roles ...string) gin.HandlerFunc {
	allowed := normalizeRoles(roles)
	return func(c *gin.Context) {
		if len(allowed) == 0 {
			c.Next()
			return
		}

		userID, ok := s.userIDFromSession(c)
		if !ok {
			AbortWithError(c, ErrUnauthorized)
			return
		}

		orgID, err := s.orgIDFromRequest(c)
		if err != nil {
			AbortWithError(c, err)
			return
		}

		role, err := s.roleForOrg(c.Request.Context(), orgID, userID)
		if err != nil {
			AbortWithError(c, err)
			return
		}

		if _, ok := allowed[role]; !ok {
			AbortWithError(c, ErrForbidden)
			return
		}

		c.Next()
	}
}

func (s *Server) roleForOrg(ctx context.Context, orgID, userID snowflake.ID) (string, error) {
	var row struct {
		Role string `gorm:"column:role"`
	}
	if err := s.db.WithContext(ctx).Raw(
		`SELECT role FROM organization_members WHERE org_id = ? AND user_id = ?`,
		orgID,
		userID,
	).Scan(&row).Error; err != nil {
		return "", err
	}

	role := normalizeRole(row.Role)
	if role == "" {
		return "", ErrForbidden
	}
	return role, nil
}

func (s *Server) orgIDFromRequest(c *gin.Context) (snowflake.ID, error) {
	if orgID, ok := orgcontext.OrgIDFromContext(c.Request.Context()); ok && orgID != 0 {
		return orgID, nil
	}

	candidate := strings.TrimSpace(c.Param("id"))
	if candidate == "" {
		candidate = strings.TrimSpace(c.Param("orgId"))
	}
	if candidate == "" {
		candidate = strings.TrimSpace(c.Param("org_id"))
	}
	if candidate == "" {
		candidate = strings.TrimSpace(c.GetHeader(HeaderOrg))
	}
	if candidate == "" {
		return 0, ErrOrgRequired
	}

	orgID, err := snowflake.ParseString(candidate)
	if err != nil {
		return 0, newValidationError("org_id", "invalid_org_id", "invalid org id")
	}
	return orgID, nil
}

func normalizeRoles(roles []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		normalized := normalizeRole(role)
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	return allowed
}

func normalizeRole(role string) string {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	if strings.HasPrefix(normalized, "ORG_") {
		normalized = strings.TrimPrefix(normalized, "ORG_")
	}
	return normalized
}

func (s *Server) sessionFromContext(c *gin.Context) (*authdomain.Session, bool) {
	value, ok := c.Get(contextSessionKey)
	if !ok {
		return nil, false
	}
	session, ok := value.(*authdomain.Session)
	return session, ok
}

func (s *Server) loadUserOrgIDs(ctx context.Context, userID snowflake.ID) ([]int64, error) {
	orgs, err := s.organizationSvc.ListOrganizationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	orgIDs := make([]int64, 0, len(orgs))
	for _, org := range orgs {
		parsed, err := snowflake.ParseString(org.ID)
		if err != nil {
			return nil, ErrInternal
		}
		orgIDs = append(orgIDs, int64(parsed))
	}

	return orgIDs, nil
}

func containsOrgID(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
