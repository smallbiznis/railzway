package tracing

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

var sensitiveAttributeKeys = []string{
	"password",
	"secret",
	"token",
	"api_key",
	"webhook_secret",
	"authorization",
}

// SafeAttributes drops attributes with sensitive keys.
func SafeAttributes(attrs ...attribute.KeyValue) []attribute.KeyValue {
	filtered := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		if isSensitiveKey(string(attr.Key)) {
			continue
		}
		filtered = append(filtered, attr)
	}
	return filtered
}

// SafeError replaces an error with a type-only error to avoid leaking details.
func SafeError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%T", err)
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, needle := range sensitiveAttributeKeys {
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}
