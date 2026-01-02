package masking

import "strings"

const maskToken = "****"

// MaskSecret redacts a secret while keeping a minimal suffix for auditing.
func MaskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	prefix, remainder := splitPrefix(trimmed)
	if len(remainder) <= 4 {
		return prefix + maskToken
	}

	return prefix + maskToken + remainder[len(remainder)-4:]
}

// MaskJSON returns a copy of the input with string values masked.
func MaskJSON(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}

	masked := make(map[string]any, len(input))
	for key, value := range input {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		masked[trimmedKey] = maskValue(value)
	}

	if len(masked) == 0 {
		return nil
	}
	return masked
}

func maskValue(value any) any {
	switch cast := value.(type) {
	case string:
		return MaskSecret(cast)
	case map[string]any:
		return MaskJSON(cast)
	case []any:
		out := make([]any, 0, len(cast))
		for _, item := range cast {
			out = append(out, maskValue(item))
		}
		return out
	default:
		return value
	}
}

func splitPrefix(value string) (string, string) {
	lastUnderscore := strings.LastIndex(value, "_")
	if lastUnderscore == -1 || lastUnderscore == len(value)-1 {
		return "", value
	}
	return value[:lastUnderscore+1], value[lastUnderscore+1:]
}
