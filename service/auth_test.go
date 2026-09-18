package service

import (
	"context"
	"testing"
	"time"

	"MCP-Nexus/model"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func newAuthFixture(t *testing.T) (*AuthService, *fakeUserRepository) {
	t.Helper()
	repo := &fakeUserRepository{users: map[string]*model.User{}}
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash 失败: %v", err)
	}
	_ = repo.Create(context.Background(), &model.User{Username: "agent", PasswordHash: string(hash), Role: "agent", Status: "active"})
	_ = repo.Create(context.Background(), &model.User{Username: "norole", PasswordHash: string(hash), Status: "active"})
	_ = repo.Create(context.Background(), &model.User{Username: "disabled", PasswordHash: string(hash), Role: "agent", Status: "banned"})
	return NewAuthService(repo, "test-secret", time.Hour), repo
}

func TestLoginSuccess(t *testing.T) {
	svc, _ := newAuthFixture(t)
	result, err := svc.Login(context.Background(), "agent", "secret123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token == "" || result.TokenType != "Bearer" {
		t.Fatalf("Login() token = %q, type = %q", result.Token, result.TokenType)
	}
	if result.User.PasswordHash != "" {
		t.Fatalf("响应泄露 password_hash: %q", result.User.PasswordHash)
	}
	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(result.Token, claims, func(t *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	}); err != nil {
		t.Fatalf("签发的令牌解析失败: %v", err)
	}
	if claims.UserID != 1 || claims.Username != "agent" || claims.Role != "agent" {
		t.Fatalf("claims = %+v", claims)
	}
	if !claims.ExpiresAt.After(time.Now()) {
		t.Fatalf("令牌已过期: %v", claims.ExpiresAt)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	svc, _ := newAuthFixture(t)
	ctx := context.Background()
	// 密码错误与用户不存在返回同一错误，不泄露账号存在性
	if _, err := svc.Login(ctx, "agent", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("错误密码 error = %v", err)
	}
	if _, err := svc.Login(ctx, "ghost", "secret123"); err != ErrInvalidCredentials {
		t.Fatalf("未知用户 error = %v", err)
	}
	if _, err := svc.Login(ctx, "  ", "secret123"); err != ErrInvalidCredentials {
		t.Fatalf("空用户名 error = %v", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	svc, _ := newAuthFixture(t)
	if _, err := svc.Login(context.Background(), "disabled", "secret123"); err != ErrUserDisabled {
		t.Fatalf("禁用用户 error = %v, want %v", err, ErrUserDisabled)
	}
}

func TestLoginUserWithoutRole(t *testing.T) {
	svc, _ := newAuthFixture(t)
	result, err := svc.Login(context.Background(), "norole", "secret123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(result.Token, claims, func(t *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	}); err != nil {
		t.Fatalf("令牌解析失败: %v", err)
	}
	if claims.Role != "" {
		t.Fatalf("无角色用户 claims.Role = %q, want 空", claims.Role)
	}
}
