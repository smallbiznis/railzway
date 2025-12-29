package local

import (
	"context"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/auth/session"
	"github.com/smallbiznis/valora/internal/config"
	orgdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Handler manages local auth endpoints.
type Handler struct {
	authsvc  authdomain.Service
	sessions *session.Manager
	log      *zap.Logger
	cfg      config.Config
	db       *gorm.DB
	genID    *snowflake.Node
}

func NewHandler(authsvc authdomain.Service, sessions *session.Manager, log *zap.Logger, cfg config.Config, db *gorm.DB, genID *snowflake.Node) *Handler {
	return &Handler{
		authsvc:  authsvc,
		sessions: sessions,
		log:      log.Named("auth.local.handler"),
		cfg:      cfg,
		db:       db,
		genID:    genID,
	}
}

func RegisterRoutes(r *gin.Engine, h *Handler) {
	group := r.Group("/internal/auth/local")
	group.POST("/signup", h.Signup)
	group.POST("/login", h.Login)
	group.POST("/logout", h.Logout)
}

type signupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Provider    string `json:"provider"`
	ExternalID  string `json:"external_id"`
}

func (h *Handler) Signup(c *gin.Context) {
	if h.cfg.IsCloud() || !h.cfg.Bootstrap.AllowSignUp {
		writeLocalError(c, http.StatusForbidden, "signup_disabled")
		return
	}

	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeLocalError(c, http.StatusBadRequest, "invalid_request")
		return
	}

	email, err := normalizeEmail(req.Email)
	if err != nil {
		writeLocalError(c, http.StatusBadRequest, "invalid_email")
		return
	}
	if len(strings.TrimSpace(req.Password)) < 8 {
		writeLocalError(c, http.StatusBadRequest, "weak_password")
		return
	}

	user, err := h.authsvc.CreateUser(c.Request.Context(), authdomain.CreateUserRequest{
		Email:       email,
		Password:    req.Password,
		DisplayName: strings.TrimSpace(req.DisplayName),
	})
	if err != nil {
		if err == authdomain.ErrUserExists {
			writeLocalError(c, http.StatusConflict, "user_exists")
			return
		}
		writeLocalError(c, http.StatusBadRequest, "invalid_request")
		return
	}

	if err := h.ensureAutoOrgMembership(c.Request.Context(), user.ID); err != nil {
		writeLocalError(c, http.StatusBadRequest, "invalid_request")
		return
	}

	sessionResult, err := h.authsvc.Login(c.Request.Context(), authdomain.LoginRequest{
		Email:     email,
		Password:  req.Password,
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		writeLocalError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	h.sessions.Set(c, sessionResult.RawToken, sessionResult.ExpiresAt)

	h.log.Info("local signup created session",
		zap.String("request_id", requestID(c)),
		zap.String("user_id", user.ID.String()),
	)

	c.JSON(http.StatusCreated, userResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Provider:    user.Provider,
		ExternalID:  user.ExternalID,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeLocalError(c, http.StatusBadRequest, "invalid_request")
		return
	}

	email, err := normalizeEmail(req.Email)
	if err != nil {
		writeLocalError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	result, err := h.authsvc.Login(c.Request.Context(), authdomain.LoginRequest{
		Email:     email,
		Password:  req.Password,
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		writeLocalError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	h.sessions.Set(c, result.RawToken, result.ExpiresAt)

	h.log.Info("local login created session",
		zap.String("request_id", requestID(c)),
	)

	c.JSON(http.StatusOK, result.Session)
}

func (h *Handler) Logout(c *gin.Context) {
	token, ok := h.sessions.ReadToken(c)
	if !ok {
		writeLocalError(c, http.StatusUnauthorized, "invalid_session")
		return
	}
	if err := h.authsvc.Logout(c.Request.Context(), token); err != nil {
		writeLocalError(c, http.StatusUnauthorized, "invalid_session")
		return
	}

	h.sessions.Clear(c)
	c.Status(http.StatusNoContent)
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}

func writeLocalError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func requestID(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Request-Id")); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.GetHeader("X-Request-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.GetString("request_id")); v != "" {
		return v
	}
	return ""
}

func (h *Handler) ensureAutoOrgMembership(ctx context.Context, userID snowflake.ID) error {
	if h.cfg.IsCloud() {
		return nil
	}
	cfg := h.cfg.Bootstrap
	if !cfg.AllowSignUp || !cfg.AllowAssignOrg {
		return nil
	}
	orgIDRaw := strings.TrimSpace(cfg.AutoAssignOrgID)
	role := strings.ToUpper(strings.TrimSpace(cfg.AutoAssignOrgRole))
	if orgIDRaw == "" || role == "" {
		return nil
	}
	if !roleAllowed(cfg.AllowAssignUserRole, role) {
		return nil
	}

	orgID, err := snowflake.ParseString(orgIDRaw)
	if err != nil {
		return err
	}

	var org orgdomain.Organization
	if err := h.db.WithContext(ctx).First(&org, "id = ?", orgID).Error; err != nil {
		return err
	}

	var count int64
	if err := h.db.WithContext(ctx).
		Model(&orgdomain.OrganizationMember{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	member := orgdomain.OrganizationMember{
		ID:        h.genID.Generate(),
		OrgID:     orgID,
		UserID:    userID,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.db.WithContext(ctx).Create(&member).Error; err != nil {
		return err
	}
	return nil
}

func roleAllowed(allowedRaw string, role string) bool {
	allowedRaw = strings.TrimSpace(allowedRaw)
	if allowedRaw == "" || role == "" {
		return false
	}
	parts := strings.FieldsFunc(allowedRaw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	for _, part := range parts {
		if strings.ToUpper(strings.TrimSpace(part)) == role {
			return true
		}
	}
	return false
}
