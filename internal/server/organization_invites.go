package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
