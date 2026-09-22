package service

import (
	"testing"
)

func TestSanitizeValueMasksSensitiveKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"密码", "password"}, {"大写TOKEN", "TOKEN"}, {"下划线api_key", "api_key"},
		{"驼峰apiKey", "apiKey"}, {"连字符access-key", "access-key"}, {"authorization", "Authorization"},
		{"secret", "client_secret"}, {"credential", "credentials"}, {"private_key", "PRIVATE_KEY"},
	}
	for _, tc := range cases {
		out := SanitizeValue(map[string]any{tc.key: "raw-secret-value"})
		got := out.(map[string]any)[tc.key]
		if got != maskValue {
			t.Fatalf("%s: 期望掩码，实际 %v", tc.name, got)
		}
	}
}

func TestSanitizeValueKeepsNormalKeys(t *testing.T) {
	in := map[string]any{"month": "2026-08", "region": "华东", "count": 3}
	out := SanitizeValue(in).(map[string]any)
	if out["month"] != "2026-08" || out["region"] != "华东" || out["count"] != 3 {
		t.Fatalf("非敏感字段不应被改动: %v", out)
	}
}

func TestSanitizeValueNested(t *testing.T) {
	in := map[string]any{
		"auth": map[string]any{"password": "abc", "note": "keep"},
		"items": []any{
			map[string]any{"token": "t1", "name": "n1"},
			"plain",
		},
		"passwordNote": "masked-by-key",
	}
	out := SanitizeValue(in).(map[string]any)
	auth := out["auth"].(map[string]any)
	if auth["password"] != maskValue || auth["note"] != "keep" {
		t.Fatalf("嵌套 map 脱敏错误: %v", auth)
	}
	items := out["items"].([]any)
	first := items[0].(map[string]any)
	if first["token"] != maskValue || first["name"] != "n1" {
		t.Fatalf("slice 内嵌套脱敏错误: %v", first)
	}
	if items[1] != "plain" {
		t.Fatalf("标量应原样: %v", items[1])
	}
	if out["passwordNote"] != maskValue {
		t.Fatalf("key 含敏感词即掩码: %v", out["passwordNote"])
	}
}

func TestIsSensitiveKey(t *testing.T) {
	if !isSensitiveKey("db_password") || isSensitiveKey("username") {
		t.Fatal("isSensitiveKey 判断错误")
	}
}
