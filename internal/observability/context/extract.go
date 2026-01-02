package context

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequestIDFromGin(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if ctx := c.Request.Context(); ctx != nil {
		if value := RequestIDFromContext(ctx); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(c.GetString("request_id")); value != "" {
		return value
	}
	return ""
}

func OrgIDFromGin(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if ctx := c.Request.Context(); ctx != nil {
		if value := OrgIDFromContext(ctx); value != "" {
			return value
		}
	}
	if raw, ok := c.Get("org_id"); ok {
		switch value := raw.(type) {
		case string:
			return strings.TrimSpace(value)
		case int64:
			if value != 0 {
				return strconv.FormatInt(value, 10)
			}
		}
	}
	return ""
}

func ActorFromGin(c *gin.Context) (string, string) {
	if c == nil {
		return "", ""
	}
	return ActorFromContext(c.Request.Context())
}
