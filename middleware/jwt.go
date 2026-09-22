package middleware

import (
	"MCP-Nexus/model"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-in-production"
	}
	jwtSecret = []byte(secret)
}

// JWTClaims carries the user identity inside a JWT.
type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a JWT for the given user.
func GenerateToken(userID int64, username, role string) (string, error) {
	expiration := os.Getenv("JWT_EXPIRATION")
	if expiration == "" {
		expiration = "24h"
	}
	d, err := time.ParseDuration(expiration)
	if err != nil {
		d = 24 * time.Hour
	}
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(d)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", errors.New("failed to sign token")
	}
	return signed, nil
}

// VerifyToken parses and validates a JWT, returning the claims.
func VerifyToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// AuthRequired is a Gin middleware that validates the Bearer token and injects user info.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		tokenStr, ok := ExtractBearer(auth)
		if !ok || tokenStr == "" {
			c.JSON(http.StatusUnauthorized, model.APIResponse{
				Code:      "UNAUTHORIZED",
				Message:   "缺少认证 Token",
				RequestID: c.GetString("request_id"),
			})
			c.Abort()
			return
		}
		claims, err := VerifyToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.APIResponse{
				Code:      "UNAUTHORIZED",
				Message:   "Token 无效或已过期",
				RequestID: c.GetString("request_id"),
			})
			c.Abort()
			return
		}
		c.Set(string(KeyUserID), claims.UserID)
		c.Set(string(KeyUsername), claims.Username)
		c.Set(string(KeyRole), claims.Role)
		c.Next()
	}
}

// OptionalAuth validates token if present but does not block if missing.
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		tokenStr, ok := ExtractBearer(auth)
		if !ok || tokenStr == "" {
			c.Next()
			return
		}
		claims, err := VerifyToken(tokenStr)
		if err != nil {
			c.Next()
			return
		}
		c.Set(string(KeyUserID), claims.UserID)
		c.Set(string(KeyUsername), claims.Username)
		c.Set(string(KeyRole), claims.Role)
		c.Next()
	}
}

// GetCurrentUserID safely reads the user ID from Gin context.
func GetCurrentUserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(string(KeyUserID))
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// GetCurrentUsername returns the username from Gin context.
func GetCurrentUsername(c *gin.Context) string {
	v, ok := c.Get(string(KeyUsername))
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// GetCurrentRole returns the role from Gin context.
func GetCurrentRole(c *gin.Context) string {
	v, ok := c.Get(string(KeyRole))
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
