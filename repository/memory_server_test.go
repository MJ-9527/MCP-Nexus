package repository

import (
	"context"
	"testing"

	"MCP-Nexus/model"
)

func TestMemoryServerRepositoryCreateAndFind(t *testing.T) {
	repo := NewMemoryServerRepository()
	server := &model.MCPServer{Name: "demo", Endpoint: "http://demo:9001"}

	if err := repo.Create(context.Background(), server); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if server.ID == 0 {
		t.Fatal("Create() did not assign an ID")
	}
	found, err := repo.FindByID(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.Name != server.Name || found.Endpoint != server.Endpoint {
		t.Fatalf("FindByID() = %+v, want %+v", found, server)
	}
}

func TestMemoryServerRepositoryRejectsDuplicate(t *testing.T) {
	repo := NewMemoryServerRepository()
	first := &model.MCPServer{Name: "demo", Endpoint: "http://demo:9001"}
	second := &model.MCPServer{Name: "demo", Endpoint: "http://other:9001"}

	if err := repo.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if err := repo.Create(context.Background(), second); err != ErrConflict {
		t.Fatalf("duplicate Create() error = %v, want %v", err, ErrConflict)
	}
}

func TestMemoryServerRepositoryNotFound(t *testing.T) {
	repo := NewMemoryServerRepository()
	if _, err := repo.FindByID(context.Background(), 999); err != ErrNotFound {
		t.Fatalf("FindByID() error = %v, want %v", err, ErrNotFound)
	}
	if err := repo.UpdateStatus(context.Background(), 999, "offline"); err != ErrNotFound {
		t.Fatalf("UpdateStatus() error = %v, want %v", err, ErrNotFound)
	}
}
