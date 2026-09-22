package middleware

import "strings"

// Context key names for Gin context values.
type contextKey string

const (
	KeyUserID    contextKey = "user_id"
	KeyUsername  contextKey = "username"
	KeyRole      contextKey = "role"
)

// ExtractBearer extracts the Bearer token from the Authorization header.
func ExtractBearer(authorization string) (string, bool) {
	if !strings.HasPrefix(authorization, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}
