package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type listAuditLogsQuery struct {
	PageToken    string `form:"page_token"`
	PageSize     int    `form:"page_size"`
	Action       string `form:"action"`
	TargetType   string `form:"target_type"`
	TargetID     string `form:"target_id"`
	ResourceType string `form:"resource_type"`
	ResourceID   string `form:"resource_id"`
	ActorType    string `form:"actor_type"`
	StartAt      string `form:"start_at"`
	EndAt        string `form:"end_at"`
	From         string `form:"from"`
	To           string `form:"to"`
}

func (s *Server) ListAuditLogs(c *gin.Context) {
	var query listAuditLogsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		AbortWithError(c, invalidRequestError())
		return
	}

	startAtValue := strings.TrimSpace(query.StartAt)
	if startAtValue == "" {
		startAtValue = strings.TrimSpace(query.From)
	}

	var startAt *time.Time
	if startAtValue != "" {
		parsed, err := time.Parse(time.RFC3339, startAtValue)
		if err != nil {
			AbortWithError(c, newValidationError("start_at", "invalid_start_at", "invalid start_at"))
			return
		}
		startAt = &parsed
	}

	endAtValue := strings.TrimSpace(query.EndAt)
	if endAtValue == "" {
		endAtValue = strings.TrimSpace(query.To)
	}

	var endAt *time.Time
	if endAtValue != "" {
		parsed, err := time.Parse(time.RFC3339, endAtValue)
		if err != nil {
			AbortWithError(c, newValidationError("end_at", "invalid_end_at", "invalid end_at"))
			return
		}
		endAt = &parsed
	}

	targetType := strings.TrimSpace(query.TargetType)
	if targetType == "" {
		targetType = strings.TrimSpace(query.ResourceType)
	}
	targetID := strings.TrimSpace(query.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(query.ResourceID)
	}

	resp, err := s.auditSvc.List(c.Request.Context(), auditdomain.ListAuditLogRequest{
		Pagination: pagination.Pagination{
			PageToken: strings.TrimSpace(query.PageToken),
			PageSize:  query.PageSize,
		},
		Action:     strings.TrimSpace(query.Action),
		TargetType: targetType,
		TargetID:   targetID,
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
