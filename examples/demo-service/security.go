package main

import (
	"encoding/json"
	"net"
	"strings"
)

var (
	// allowedFetchHosts 是 fetch_url 工具的域名白名单，阻止任意 SSRF。
	allowedFetchHosts = map[string]bool{
		"example.com":      true,
		"postman-echo.com": true,
		"httpbin.org":      true,
	}

	// serviceAPIKey 是敏感工具的 API Key（可通过 API_KEY 环境变量覆盖）。
	// 敏感工具（如 delete_customer）必须携带正确的 X-API-Key Header 才会执行下游操作。
	serviceAPIKey = "demo-api-key"
)

// isPrivateIP 判断 IP 是否属于内网/回环/链路本地/未指定地址。
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// redactSensitive 对 JSON 响应中的敏感字段做脱敏；非 JSON 返回原始文本。
func redactSensitive(data []byte, contentType string) any {
	if strings.Contains(contentType, "application/json") {
		var v any
		if err := json.Unmarshal(data, &v); err == nil {
			return redactValue(v)
		}
	}
	return string(data)
}

// redactValue 递归脱敏 map/slice 中的敏感键。
func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSensitiveKey(k) {
				out[k] = "***"
				continue
			}
			out[k] = redactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactValue(val)
		}
		return out
	default:
		return v
	}
}

// isSensitiveKey 判断键名是否包含敏感字段关键词。
func isSensitiveKey(k string) bool {
	lower := strings.ToLower(k)
	for _, s := range []string{"password", "passwd", "token", "secret", "api_key", "apikey", "authorization", "cookie"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
