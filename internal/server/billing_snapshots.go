package server

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/smallbiznis/valora/internal/billingdashboard/rollup"
	"github.com/smallbiznis/valora/internal/orgcontext"
)

type rebuildBillingSnapshotsRequest struct {
	OrgID          string `json:"org_id"`
	BillingCycleID string `json:"billing_cycle_id"`
	Scope          string `json:"scope"`
}

func (s *Server) RebuildBillingSnapshots(c *gin.Context) {
	if s.billingRollup == nil {
		AbortWithError(c, ErrServiceUnavailable)
		return
	}

	var req rebuildBillingSnapshotsRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		AbortWithError(c, invalidRequestError())
		return
	}

	orgIDValue, ok := orgcontext.OrgIDFromContext(c.Request.Context())
	if !ok || orgIDValue == 0 {
		AbortWithError(c, ErrOrgRequired)
		return
	}
	orgID := snowflake.ID(orgIDValue)

	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	orgScope := &orgID
	if scope == "all" {
		if s.cfg.IsCloud() {
			AbortWithError(c, ErrForbidden)
			return
		}
		orgScope = nil
	}

	orgOverride, err := parseOptionalSnowflakeID(req.OrgID)
	if err != nil {
		AbortWithError(c, newValidationError("org_id", "invalid_org_id", "invalid org id"))
		return
	}
	if orgOverride != nil {
		if scope == "all" {
			AbortWithError(c, newValidationError("org_id", "unsupported_org_id", "org_id not allowed with scope=all"))
			return
		}
		if *orgOverride != orgID {
			AbortWithError(c, ErrForbidden)
			return
		}
		orgScope = orgOverride
	}

	cycleID, err := parseOptionalSnowflakeID(req.BillingCycleID)
	if err != nil {
		AbortWithError(c, newValidationError("billing_cycle_id", "invalid_billing_cycle_id", "invalid billing cycle id"))
		return
	}

	jobID, err := s.billingRollup.EnqueueRebuild(c.Request.Context(), rollup.RebuildRequest{
		OrgID:          orgScope,
		BillingCycleID: cycleID,
	})
	if err != nil {
		AbortWithError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id": jobID,
		"status": "queued",
	})
}
