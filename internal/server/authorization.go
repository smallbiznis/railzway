package server

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
)

func (s *Server) authorizeOrgAction(c *gin.Context, object string, action string) error {
	if s.authzSvc == nil {
		return ErrForbidden
	}
	userID, ok := s.userIDFromSession(c)
	if !ok {
		return ErrUnauthorized
	}
	orgID, err := s.orgIDFromRequest(c)
	if err != nil {
		return err
	}
	return s.authorizeForOrg(c, userID, orgID, object, action)
}

func (s *Server) authorizeForOrg(c *gin.Context, userID snowflake.ID, orgID snowflake.ID, object string, action string) error {
	actor := fmt.Sprintf("user:%s", userID.String())
	return s.authzSvc.Authorize(c.Request.Context(), actor, orgID.String(), strings.TrimSpace(object), strings.TrimSpace(action))
}
