package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func runRBACRequest(t *testing.T, currentRole string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 模拟 JWT 中间件（B5）已注入角色上下文
	r.Use(func(c *gin.Context) { c.Set("agent_role", currentRole); c.Next() })
	r.GET("/probe", handler, func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequireRoleAllowed(t *testing.T) {
	w := runRBACRequest(t, "admin", RequireRole("admin"))
	if w.Code != http.StatusOK {
		t.Fatalf("admin 角色应放行，实际 %d", w.Code)
	}
}

func TestRequireRoleDenied(t *testing.T) {
	for _, role := range []string{"agent", "developer", ""} {
		w := runRBACRequest(t, role, RequireRole("admin"))
		if w.Code != http.StatusForbidden {
			t.Fatalf("角色 %q 应被拒，实际 %d", role, w.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["code"] != "FORBIDDEN" {
			t.Fatalf("角色 %q 响应 code = %v", role, body["code"])
		}
		if body["denied_reason"] == "" {
			t.Fatalf("角色 %q 缺少拒绝原因", role)
		}
	}
}

func TestRequireRoleMultipleRoles(t *testing.T) {
	w := runRBACRequest(t, "developer", RequireRole("admin", "developer"))
	if w.Code != http.StatusOK {
		t.Fatalf("多角色名单应放行 developer，实际 %d", w.Code)
	}
}
