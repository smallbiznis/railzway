package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	authdomain "github.com/smallbiznis/railzway/internal/auth/domain"
	organizationdomain "github.com/smallbiznis/railzway/internal/organization/domain"
)

type inviteMembersRequest struct {
	Invites []inviteMemberRequest `json:"invites"`
}

type inviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type billingPreferencesRequest struct {
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
}

func (s *Server) InviteOrganizationMembers(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	orgID := strings.TrimSpace(c.Param("id"))
	if orgID == "" {
		AbortWithError(c, organizationdomain.ErrInvalidOrganization)
		return
	}

	var req inviteMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	if len(req.Invites) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	invites := make([]organizationdomain.InviteRequest, 0, len(req.Invites))
	for _, invite := range req.Invites {
		invites = append(invites, organizationdomain.InviteRequest{
			Email: invite.Email,
			Role:  invite.Role,
		})
	}

	if err := s.organizationSvc.InviteMembers(c.Request.Context(), userID, orgID, invites); err != nil {
		AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) SetOrganizationBillingPreferences(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	orgID := strings.TrimSpace(c.Param("id"))
	if orgID == "" {
		AbortWithError(c, organizationdomain.ErrInvalidOrganization)
		return
	}

	var req billingPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	if err := s.organizationSvc.SetBillingPreferences(c.Request.Context(), userID, orgID, organizationdomain.BillingPreferencesRequest{
		Currency: req.Currency,
		Timezone: req.Timezone,
	}); err != nil {
		AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) AcceptOrganizationInvite(c *gin.Context) {
	userID, ok := s.userIDFromSession(c)
	if !ok {
		AbortWithError(c, ErrUnauthorized)
		return
	}

	inviteID := strings.TrimSpace(c.Param("invite_id"))
	if inviteID == "" {
		AbortWithError(c, invalidRequestError())
		return
	}

	if err := s.organizationSvc.AcceptInvite(c.Request.Context(), userID, inviteID); err != nil {
		AbortWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) GetPublicInviteInfo(c *gin.Context) {
	inviteID := strings.TrimSpace(c.Param("invite_id"))
	if inviteID == "" {
		AbortWithError(c, invalidRequestError())
		return
	}

	info, err := s.organizationSvc.GetInvite(c.Request.Context(), inviteID)
	if err != nil {
		if err == organizationdomain.ErrInvalidOrganization {
			AbortWithError(c, ErrNotFound)
			return
		}
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, info)
}

type completeInviteRequest struct {
	Password string `json:"password"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

func (s *Server) CompleteInvite(c *gin.Context) {
	inviteID := strings.TrimSpace(c.Param("invite_id"))
	if inviteID == "" {
		AbortWithError(c, invalidRequestError())
		return
	}

	var req completeInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	// 1. Get Invite details to verify existence and get email
	info, err := s.organizationSvc.GetInvite(c.Request.Context(), inviteID)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	// 2. Create User (if password provided)
	// We assume if they call CompleteInvite, they need to create a user.
	// If they already have a user, they should have logged in and used the standard AcceptInvite.
	createUserReq := authdomain.CreateUserRequest{
		Email:       info.Email,
		Password:    req.Password,
		DisplayName: req.Name,
		Username:    req.Username,
	}

	user, err := s.authsvc.CreateUser(c.Request.Context(), createUserReq)
	if err != nil {
		AbortWithError(c, err)
		return
	}

	// 3. Accept Invite using the new UserID
	if err := s.organizationSvc.AcceptInvite(c.Request.Context(), user.ID, inviteID); err != nil {
		AbortWithError(c, err)
		return
	}

	// 4. Create Session for the new user (Auto-login)
	session, err := s.authsvc.Login(c.Request.Context(), authdomain.LoginRequest{
		Email:     info.Email,
		Password:  req.Password,
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		// This shouldn't happen if CreateUser and AcceptInvite succeeded
		AbortWithError(c, err)
		return
	}

	if session.Session != nil && session.RawToken != "" {
		s.sessions.Set(c, session.RawToken, session.ExpiresAt)
		s.enrichSessionFromToken(c, session.Session, session.RawToken)
	}

	c.JSON(http.StatusOK, session.Session)
}
