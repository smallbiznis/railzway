package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	"github.com/smallbiznis/valora/internal/authorization"
	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type listAuditLogsQuery struct {
	PageToken  string `form:"page_token"`
	PageSize   int    `form:"page_size"`
	Action     string `form:"action"`
	TargetType string `form:"target_type"`
	TargetID   string `form:"target_id"`
	ActorType  string `form:"actor_type"`
	StartAt    string `form:"start_at"`
	EndAt      string `form:"end_at"`
}

func (s *Server) ListAuditLogs(c *gin.Context) {
	if err := s.authorizeOrgAction(c, authorization.ObjectAuditLog, authorization.ActionAuditLogView); err != nil {
		AbortWithError(c, err)
		return
	}

	var query listAuditLogsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	var startAt *time.Time
	if value := strings.TrimSpace(query.StartAt); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			AbortWithError(c, newValidationError("start_at", "invalid_start_at", "invalid start_at"))
			return
		}
		startAt = &parsed
	}

	var endAt *time.Time
	if value := strings.TrimSpace(query.EndAt); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			AbortWithError(c, newValidationError("end_at", "invalid_end_at", "invalid end_at"))
			return
		}
		endAt = &parsed
	}

	resp, err := s.auditSvc.List(c.Request.Context(), auditdomain.ListAuditLogRequest{
		Pagination: pagination.Pagination{
			PageToken: strings.TrimSpace(query.PageToken),
			PageSize:  query.PageSize,
		},
		Action:     strings.TrimSpace(query.Action),
		TargetType: strings.TrimSpace(query.TargetType),
		TargetID:   strings.TrimSpace(query.TargetID),
		ActorType:  strings.TrimSpace(query.ActorType),
		StartAt:    startAt,
		EndAt:      endAt,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resp.AuditLogs, "page_info": resp.PageInfo})
}
