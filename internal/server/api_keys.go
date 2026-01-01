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
	authscope "github.com/smallbiznis/valora/internal/auth/scope"
	"gorm.io/gorm"
)

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type revealAPIKeyRequest struct {
	Password string `json:"password"`
}

func (s *Server) ListAPIKeys(c *gin.Context) {
	keys, err := s.apiKeySvc.List(c.Request.Context())
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, keys)
}

func (s *Server) CreateAPIKey(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	scopes := authscope.Normalize(req.Scopes)
	if err := authscope.Validate(scopes); err != nil {
		AbortWithError(c, err)
		return
	}

	resp, err := s.apiKeySvc.Create(c.Request.Context(), apikeydomain.CreateRequest{Name: req.Name, Scopes: scopes})
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

func (s *Server) ListAPIKeyScopes(c *gin.Context) {
	scopes := authscope.All()
	c.JSON(http.StatusOK, scopes)
}

// RevealAPIKey reveals the API key for a user. It requires the user to confirm their password.
func (s *Server) RevealAPIKey(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
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
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, "", nil, "api_key.reveal", "api_key", &targetID, map[string]any{
			"reveal_from_key_id": keyID,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) RevokeAPIKey(c *gin.Context) {
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
