package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

func newTestHealthService(endpoint string) *HealthCheckService {
	repo := repository.NewMemoryServerRepository()
	now := time.Now()
	_ = repo.Create(context.Background(), &model.MCPServer{
		ID: 1, Name: "test-server", Endpoint: endpoint, Version: "1.0.0",
		Status: "active", HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
	})
	s := NewHealthCheckService(repo, client.NewHealthClient(5*time.Second))
	s.now = func() time.Time { return now }
	return s
}

func TestHealthCheckOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestHealthService(srv.URL)
	res, err := s.HealthCheck(context.Background(), 1)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if res.HealthStatus != "online" {
		t.Errorf("health_status = %s, want online", res.HealthStatus)
	}
	if res.LatencyMS < 0 {
		t.Errorf("latency_ms < 0: %d", res.LatencyMS)
	}
}

func TestHealthCheckDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestHealthService(srv.URL)
	res, err := s.HealthCheck(context.Background(), 1)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if res.HealthStatus != "degraded" {
		t.Errorf("health_status = %s, want degraded", res.HealthStatus)
	}
}

func TestHealthCheckServerNotFound(t *testing.T) {
	s := newTestHealthService("http://127.0.0.1:1")
	_, err := s.HealthCheck(context.Background(), 999)
	if !errors.Is(err, ErrHealthServerNotFound) {
		t.Fatalf("err = %v, want ErrHealthServerNotFound", err)
	}
}

func TestStatusFromErr(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "online"},
		{client.ErrUnhealthy, "degraded"},
		{errors.New("timeout"), "offline"},
	}
	for _, c := range cases {
		if got := statusFromErr(c.err); got != c.want {
			t.Errorf("statusFromErr(%v) = %s, want %s", c.err, got, c.want)
		}
	}
}
