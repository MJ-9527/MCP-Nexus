package service

import (
	"context"
	"errors"
	"strings"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidUser  = errors.New("invalid user")
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
)

type UserService struct{ users repository.UserRepository }

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) Create(ctx context.Context, user *model.User) error {
	if s == nil || s.users == nil || user == nil || strings.TrimSpace(user.Username) == "" || user.PasswordHash == "" {
		return ErrInvalidUser
	}
	user.Username = strings.TrimSpace(user.Username)
	if user.Status == "" {
		user.Status = "active"
	}
	if err := s.users.Create(ctx, user); errors.Is(err, repository.ErrConflict) {
		return ErrUserExists
	} else {
		return err
	}
}

func (s *UserService) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	if s == nil || s.users == nil || strings.TrimSpace(username) == "" {
		return nil, ErrInvalidUser
	}
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}
