package service

import (
	"context"
	"testing"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

type fakeUserRepository struct {
	users map[string]*model.User
}

func (f *fakeUserRepository) Create(_ context.Context, user *model.User) error {
	if _, ok := f.users[user.Username]; ok {
		return repository.ErrConflict
	}
	user.ID = 1
	f.users[user.Username] = user
	return nil
}
func (f *fakeUserRepository) FindByUsername(_ context.Context, username string) (*model.User, error) {
	user, ok := f.users[username]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return user, nil
}
func (f *fakeUserRepository) CreateRole(context.Context, *model.Role) error  { return nil }
func (f *fakeUserRepository) AssignRole(context.Context, int64, int64) error { return nil }

func TestUserServiceCreateAndFind(t *testing.T) {
	repo := &fakeUserRepository{users: map[string]*model.User{}}
	svc := NewUserService(repo)
	user := &model.User{Username: " alice ", PasswordHash: "hash"}
	if err := svc.Create(context.Background(), user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	found, err := svc.FindByUsername(context.Background(), "alice")
	if err != nil || found.Username != "alice" || found.PasswordHash != "hash" {
		t.Fatalf("FindByUsername() = %+v, %v", found, err)
	}
}

func TestUserServiceMapsErrors(t *testing.T) {
	repo := &fakeUserRepository{users: map[string]*model.User{}}
	svc := NewUserService(repo)
	user := &model.User{Username: "alice", PasswordHash: "hash"}
	_ = svc.Create(context.Background(), user)
	if err := svc.Create(context.Background(), &model.User{Username: "alice", PasswordHash: "hash"}); err != ErrUserExists {
		t.Fatalf("duplicate Create() error = %v, want %v", err, ErrUserExists)
	}
	if _, err := svc.FindByUsername(context.Background(), "missing"); err != ErrUserNotFound {
		t.Fatalf("missing FindByUsername() error = %v, want %v", err, ErrUserNotFound)
	}
}
