package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type testCleanupRequest struct {
	Prefix string `json:"prefix"`
}

func (s *Server) TestCleanup(c *gin.Context) {
	if s.cfg.Environment == "production" {
		AbortWithError(c, ErrNotFound)
		return
	}

	var req testCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	prefix := strings.TrimSpace(req.Prefix)
	if prefix == "" {
		AbortWithError(c, newValidationError("prefix", "required", "prefix is required"))
		return
	}

	ctx := c.Request.Context()
	orgIDs, err := s.loadOrgIDsByPrefix(ctx, prefix)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if err := s.deleteOrgData(ctx, orgIDs); err != nil {
		AbortWithError(c, err)
		return
	}

	userIDs, err := s.loadUserIDsByPrefix(ctx, prefix)
	if err != nil {
		AbortWithError(c, err)
		return
	}
	if err := s.deleteUserData(ctx, userIDs); err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) loadOrgIDsByPrefix(ctx context.Context, prefix string) ([]int64, error) {
	like := strings.TrimSpace(prefix) + "%"
	var orgIDs []int64
	if err := s.db.WithContext(ctx).
		Table("organizations").
		Select("id").
		Where("name LIKE ?", like).
		Scan(&orgIDs).Error; err != nil {
		return nil, err
	}
	return orgIDs, nil
}

func (s *Server) deleteOrgData(ctx context.Context, orgIDs []int64) error {
	if len(orgIDs) == 0 {
		return nil
	}
	queries := []string{
		`DELETE FROM price_tiers WHERE org_id IN ?`,
		`DELETE FROM price_amounts WHERE org_id IN ?`,
		`DELETE FROM prices WHERE org_id IN ?`,
		`DELETE FROM products WHERE org_id IN ?`,
		`DELETE FROM customers WHERE org_id IN ?`,
		`DELETE FROM organization_members WHERE org_id IN ?`,
		`DELETE FROM organizations WHERE id IN ?`,
	}
	for _, query := range queries {
		if err := s.db.WithContext(ctx).Exec(query, orgIDs).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) loadUserIDsByPrefix(ctx context.Context, prefix string) ([]int64, error) {
	like := strings.TrimSpace(prefix) + "%"
	var userIDs []int64
	if err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username LIKE ?", like).
		Scan(&userIDs).Error; err != nil {
		return nil, err
	}
	return userIDs, nil
}

func (s *Server) deleteUserData(ctx context.Context, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	queries := []string{
		`DELETE FROM sessions WHERE user_id IN ?`,
		`DELETE FROM organization_members WHERE user_id IN ?`,
		`DELETE FROM users WHERE id IN ?`,
	}
	for _, query := range queries {
		if err := s.db.WithContext(ctx).Exec(query, userIDs).Error; err != nil {
			return err
		}
	}
	return nil
}
