package logger

import (
	"net/http"
	"strings"
)

var sensitiveKeys = []string{
	"password",
	"secret",
	"token",
	"api_key",
	"webhook_secret",
	"authorization",
}

// MaskAuthorization masks bearer tokens, preserving the scheme.
func MaskAuthorization(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Fields(value)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return "Bearer " + maskLast4(parts[1])
	}
	return maskLast4(value)
}

// MaskCookie masks cookie values while preserving cookie names.
func MaskCookie(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ";")
	masked := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := strings.TrimSpace(part)
		if segment == "" {
			continue
		}
		if idx := strings.Index(segment, "="); idx >= 0 {
			key := strings.TrimSpace(segment[:idx])
			val := strings.TrimSpace(segment[idx+1:])
			segment = key + "=" + maskLast4(val)
		} else {
			segment = maskLast4(segment)
		}
		masked = append(masked, segment)
	}
	return strings.Join(masked, "; ")
}

// MaskAPIKey masks API keys, preserving only the last 4 characters.
func MaskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return maskLast4(value)
}

// MaskHeaders returns a copy of headers with sensitive fields masked.
func MaskHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	masked := make(map[string]string, len(headers))
	for key, values := range headers {
		joined := strings.Join(values, ",")
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "authorization":
			masked[key] = MaskAuthorization(joined)
		case "cookie":
			masked[key] = MaskCookie(joined)
		default:
			masked[key] = joined
		}
	}
	return masked
}

// MaskJSON returns a deep-copied map with sensitive fields masked.
func MaskJSON(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		if isSensitiveKey(key) {
			out[key] = maskValue(value)
			continue
		}
		out[key] = maskJSONValue(value)
	}
	return out
}

// SafeFieldsFromRequest returns masked headers and safe request metadata.
func SafeFieldsFromRequest(req *http.Request) map[string]any {
	if req == nil {
		return map[string]any{}
	}
	fields := map[string]any{
		"method":         req.Method,
		"path":           req.URL.Path,
		"content_length": maxInt64(req.ContentLength, 0),
		"headers":        MaskHeaders(req.Header),
	}
	return fields
}

func maskJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return MaskJSON(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, entry := range typed {
			items = append(items, maskJSONValue(entry))
		}
		return items
	default:
		return value
	}
}

func maskValue(value any) any {
	switch typed := value.(type) {
	case string:
		return maskLast4(typed)
	case []byte:
		return maskLast4(string(typed))
	default:
		return "****"
	}
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, needle := range sensitiveKeys {
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}

func maskLast4(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****" + value
	}
	return "****" + value[len(value)-4:]
}

func maxInt64(value, min int64) int64 {
	if value < min {
		return min
	}
	return value
}
