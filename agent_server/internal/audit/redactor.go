package audit

import "strings"

var sensitiveKeys = []string{
	"api_key",
	"apikey",
	"authorization",
	"token",
	"secret",
	"password",
	"credential",
}

func RedactMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if isSensitiveKey(key) {
			out[key] = "[REDACTED]"
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			out[key] = RedactMap(typed)
		case map[string]string:
			out[key] = redactStringMap(typed)
		default:
			out[key] = value
		}
	}
	return out
}

func redactStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		if isSensitiveKey(key) {
			out[key] = "[REDACTED]"
		} else {
			out[key] = value
		}
	}
	return out
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}
