package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func signToken(t *testing.T, secret string, claims *service.Claims, method jwt.SigningMethod) string {
	t.Helper()
	if method == nil {
		method = jwt.SigningMethodHS256
	}
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("签发测试令牌失败: %v", err)
	}
	return token
}

func runAuthRequest(t *testing.T, secret, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("request_id", "req-test"); c.Next() })
	r.GET("/probe", JWTAuth(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":  c.GetInt64("user_id"),
			"username": c.GetString("username"),
			"role":     c.GetString("agent_role"),
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuthMissingToken(t *testing.T) {
	for _, header := range []string{"", "Basic dXNlcjpwYXNz", "Bearer "} {
		w := runAuthRequest(t, "secret", header)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("缺失令牌 Authorization=%q 状态码 = %d, want 401", header, w.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["code"] != "UNAUTHORIZED" {
			t.Fatalf("缺失令牌响应 code = %v", body["code"])
		}
	}
}

func TestJWTAuthForgedToken(t *testing.T) {
	// 用错误密钥签发的令牌（伪造）
	forged := signToken(t, "attacker-secret", &service.Claims{
		UserID: 1, Username: "agent", Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}, nil)
	w := runAuthRequest(t, "real-secret", "Bearer "+forged)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("伪造令牌状态码 = %d, want 401", w.Code)
	}
}

func TestJWTAuthExpiredToken(t *testing.T) {
	expired := signToken(t, "secret", &service.Claims{
		UserID: 1, Username: "agent", Role: "agent",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))},
	}, nil)
	w := runAuthRequest(t, "secret", "Bearer "+expired)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("过期令牌状态码 = %d, want 401", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["message"] != "令牌已过期" {
		t.Fatalf("过期令牌 message = %v", body["message"])
	}
}

func TestJWTAuthWrongAlgorithm(t *testing.T) {
	token := signToken(t, "secret", &service.Claims{
		UserID: 1, Username: "agent", Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}, jwt.SigningMethodHS512)
	w := runAuthRequest(t, "secret", "Bearer "+token)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("非约定算法状态码 = %d, want 401", w.Code)
	}
}

func TestJWTAuthValidToken(t *testing.T) {
	token := signToken(t, "secret", &service.Claims{
		UserID: 7, Username: "agent", Role: "agent",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mcp-nexus",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, nil)
	w := runAuthRequest(t, "secret", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("合法令牌状态码 = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["user_id"] != float64(7) || body["username"] != "agent" || body["role"] != "agent" {
		t.Fatalf("上下文注入 = %v", body)
	}
}
