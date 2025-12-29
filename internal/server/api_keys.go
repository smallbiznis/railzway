package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	apikeydomain "github.com/smallbiznis/valora/internal/apikey/domain"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/authorization"
	organizationdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"gorm.io/gorm"
)

type createAPIKeyRequest struct {
	Name string `json:"name"`
}

type revealAPIKeyRequest struct {
	Password string `json:"password"`
}

func (s *Server) registerAdminRoutes() {
	admin := s.engine.Group("/admin")
	admin.Use(RequestID())
	admin.Use(s.AuthRequired())
	admin.Use(s.OrgContext())

	admin.GET("/audit-logs", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListAuditLogs)
	admin.GET("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListAPIKeys)
	admin.POST("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateAPIKey)
	admin.POST("/api-keys/:key_id/reveal", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.RevealAPIKey)
	admin.POST("/api-keys/:key_id/revoke", s.RequireRole(organizationdomain.RoleOwner), s.RevokeAPIKey)
}

func (s *Server) ListAPIKeys(c *gin.Context) {
	if err := s.authorizeOrgAction(c, authorization.ObjectAPIKey, authorization.ActionAPIKeyView); err != nil {
		AbortWithError(c, err)
		return
	}

	keys, err := s.apiKeySvc.List(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, keys)
}

func (s *Server) CreateAPIKey(c *gin.Context) {
	if err := s.authorizeOrgAction(c, authorization.ObjectAPIKey, authorization.ActionAPIKeyCreate); err != nil {
		AbortWithError(c, err)
		return
	}

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	resp, err := s.apiKeySvc.Create(c.Request.Context(), apikeydomain.CreateRequest{Name: req.Name})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil && resp != nil {
		targetID := resp.KeyID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "api_key.created", "api_key", &targetID, map[string]any{
			"name": strings.TrimSpace(req.Name),
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) RevealAPIKey(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	if err := s.authorizeOrgAction(c, authorization.ObjectAPIKey, authorization.ActionAPIKeyRotate); err != nil {
		AbortWithError(c, err)
		return
	}

	if s.apiKeyLimiter != nil && !s.apiKeyLimiter.Allow(userID.String()) {
		AbortWithError(c, ErrRateLimited)
		return
	}

	var req revealAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		AbortWithError(c, newValidationError("password", "required", "password is required"))
		return
	}

	if err := s.confirmPassword(c.Request.Context(), userID, password); err != nil {
		AbortWithError(c, err)
		return
	}

	keyID := strings.TrimSpace(c.Param("key_id"))
	resp, err := s.apiKeySvc.Rotate(c.Request.Context(), keyID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil && resp != nil {
		targetID := resp.KeyID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "api_key.rotated", "api_key", &targetID, map[string]any{
			"rotated_from_key_id": keyID,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) RevokeAPIKey(c *gin.Context) {
	if err := s.authorizeOrgAction(c, authorization.ObjectAPIKey, authorization.ActionAPIKeyRevoke); err != nil {
		AbortWithError(c, err)
		return
	}

	keyID := strings.TrimSpace(c.Param("key_id"))
	if err := s.apiKeySvc.Revoke(c.Request.Context(), keyID); err != nil {
		AbortWithError(c, err)
		return
	}

	if s.auditSvc != nil {
		targetID := keyID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "api_key.revoked", "api_key", &targetID, nil)
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) confirmPassword(ctx context.Context, userID snowflake.ID, password string) error {
	var user authdomain.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	if user.Provider != "local" {
		return ErrForbidden
	}
	if user.PasswordHash == nil || !verifyPassword(password, *user.PasswordHash) {
		return authdomain.ErrInvalidCredentials
	}
	return nil
}
