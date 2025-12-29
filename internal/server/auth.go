package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) Login(c *gin.Context) {

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	email := strings.TrimSpace(req.Email)
	result, err := s.authsvc.Login(c.Request.Context(), authdomain.LoginRequest{
		Email:     email,
		Password:  req.Password,
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		if s.auditSvc != nil {
			_ = s.auditSvc.AuditLog(c.Request.Context(), nil, string(auditdomain.ActorTypeUser), nil, "user.login_failed", "user", nil, map[string]any{
				"email": email,
			})
		}
		AbortWithError(c, err)
		return
	}

	s.sessions.Set(c, result.RawToken, result.ExpiresAt)

	s.enrichSessionMetadata(c, result)

	if s.auditSvc != nil {
		var userID *string
		if result.Session != nil {
			if rawUserID, ok := result.Session.Metadata["user_id"].(string); ok && strings.TrimSpace(rawUserID) != "" {
				trimmed := strings.TrimSpace(rawUserID)
				userID = &trimmed
			}
		}
		targetID := userID
		_ = s.auditSvc.AuditLog(c.Request.Context(), nil, string(auditdomain.ActorTypeUser), userID, "user.login", "user", targetID, map[string]any{
			"email": email,
		})
	}

	c.JSON(http.StatusOK, result.Session)
}

func (s *Server) ChangePassword(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if currentPassword == "" {
		AbortWithError(c, newValidationError("current_password", "required", "current password is required"))
		return
	}
	if newPassword == "" {
		AbortWithError(c, newValidationError("new_password", "required", "new password is required"))
		return
	}
	if currentPassword == newPassword {
		AbortWithError(c, newValidationError("new_password", "must_differ", "new password must be different"))
		return
	}
	if len(newPassword) < 8 {
		AbortWithError(c, newValidationError("new_password", "weak_password", "password must be at least 8 characters"))
		return
	}

	var user authdomain.User
	if err := s.db.WithContext(c.Request.Context()).First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			AbortWithError(c, ErrUnauthorized)
			return
		}
		AbortWithError(c, err)
		return
	}
	if user.Provider != "local" {
		AbortWithError(c, ErrForbidden)
		return
	}
	if user.PasswordHash == nil || !verifyPassword(currentPassword, *user.PasswordHash) {
		AbortWithError(c, authdomain.ErrInvalidCredentials)
		return
	}

	if err := s.authsvc.ChangePassword(c.Request.Context(), userID.String(), newPassword); err != nil {
		AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) Logout(c *gin.Context) {
	token, ok := s.sessions.ReadToken(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	if err := s.authsvc.Logout(c.Request.Context(), token); err != nil {
		AbortWithError(c, err)
		return
	}

	s.sessions.Clear(c)
	c.Status(http.StatusNoContent)
}

func (s *Server) Me(c *gin.Context) {
	AbortWithError(c, ErrServiceUnavailable)
}

func (s *Server) Forgot(c *gin.Context) {
	AbortWithError(c, ErrServiceUnavailable)
}

func (s *Server) enrichSessionMetadata(c *gin.Context, result *authdomain.LoginResult) {
	if result == nil || result.Session == nil {
		return
	}

	rawUserID, ok := result.Session.Metadata["user_id"].(string)
	if !ok {
		return
	}

	parsedUserID, err := snowflake.ParseString(rawUserID)
	if err != nil {
		return
	}

	orgIDs, err := s.loadUserOrgIDs(c.Request.Context(), parsedUserID)
	if err != nil {
		return
	}

	if err := s.authsvc.UpdateSessionOrgContext(c.Request.Context(), result.SessionID, nil, orgIDs); err != nil {
		return
	}

	result.Session.Metadata["org_ids"] = toOrgIDStrings(orgIDs)
}
