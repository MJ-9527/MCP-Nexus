package service

import (
	"regexp"
	"strings"
)

// sensitiveKeyPattern 命中即视为敏感字段：password、token、secret、api_key、
// authorization、credential、private_key 等（大小写/分隔符不敏感）。
var sensitiveKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|apikey|authorization|credential|access[_-]?key|private[_-]?key)`)

// maskValue 敏感字段统一掩码值。
const maskValue = "***"

// SanitizeValue 递归脱敏：map 的 key 命中敏感模式时值替换为掩码，
// slice 逐项处理，其余原样返回。参数在落审计前必须经过本函数（B8 脱敏）。
func SanitizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for k, v := range typed {
			if sensitiveKeyPattern.MatchString(k) {
				sanitized[k] = maskValue
				continue
			}
			sanitized[k] = SanitizeValue(v)
		}
		return sanitized
	case []any:
		sanitized := make([]any, len(typed))
		for i, v := range typed {
			sanitized[i] = SanitizeValue(v)
		}
		return sanitized
	default:
		return value
	}
}

// isSensitiveKey 判断字段名是否敏感（供测试与扩展）。
func isSensitiveKey(key string) bool {
	return sensitiveKeyPattern.MatchString(strings.TrimSpace(key))
}
