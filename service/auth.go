package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user disabled")
	ErrInvalidAuthConfig  = errors.New("invalid auth config")
)

// Claims 网关 JWT 载荷（B5）：用户 ID + 主角色，供认证中间件与后续 RBAC 使用
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	users   repository.UserRepository
	secret  []byte
	ttl     time.Duration
	nowFunc func() time.Time
}

func NewAuthService(users repository.UserRepository, secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: users, secret: []byte(secret), ttl: ttl, nowFunc: time.Now}
}

type LoginResult struct {
	Token     string      `json:"token"`
	TokenType string      `json:"token_type"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      *model.User `json:"user"`
}

// Login 校验用户名密码并签发 JWT（HS256）
func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	if s == nil || s.users == nil || len(s.secret) == 0 || s.ttl <= 0 {
		return nil, ErrInvalidAuthConfig
	}
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	user, err := s.users.FindByUsername(ctx, username)
	if errors.Is(err, repository.ErrNotFound) {
		// 用户不存在与密码错误返回同一错误，避免泄露账号是否存在
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := s.nowFunc()
	expiresAt := now.Add(s.ttl)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mcp-nexus",
			Subject:   user.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return &LoginResult{Token: token, TokenType: "Bearer", ExpiresAt: expiresAt, User: user}, nil
}
