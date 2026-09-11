package service

import (
	"context"
	"testing"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

func TestServerServiceRegisterAndRejectDuplicate(t *testing.T) {
	repo := repository.NewMemoryServerRepository()
	service := NewServerService(repo)
	request := model.RegisterServerRequest{
		Name:     "demo",
		Endpoint: "http://demo:9001",
		Version:  "1.0.0",
	}

	server, err := service.Register(context.Background(), request)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if server.ID == 0 || server.Status != "draft" || server.HealthStatus != "unknown" {
		t.Fatalf("Register() returned invalid server: %+v", server)
	}

	if _, err := service.Register(context.Background(), request); err != ErrServerExists {
		t.Fatalf("duplicate Register() error = %v, want %v", err, ErrServerExists)
	}
}

func TestServerServiceRejectsInvalidEndpoint(t *testing.T) {
	service := NewServerService(repository.NewMemoryServerRepository())
	_, err := service.Register(context.Background(), model.RegisterServerRequest{
		Name:     "demo",
		Endpoint: "not-a-url",
		Version:  "1.0.0",
	})
	if err != ErrInvalidServer {
		t.Fatalf("Register() error = %v, want %v", err, ErrInvalidServer)
	}
}
