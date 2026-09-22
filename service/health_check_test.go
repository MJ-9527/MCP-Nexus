package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestHealthCheckAll(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	repo := repository.NewMemoryServerRepository()
	now := time.Now()
	_ = repo.Create(context.Background(), &model.MCPServer{
		ID: 1, Name: "ok", Endpoint: ok.URL, Version: "1.0.0",
		Status: "active", HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
	})
	_ = repo.Create(context.Background(), &model.MCPServer{
		ID: 2, Name: "bad", Endpoint: bad.URL, Version: "1.0.0",
		Status: "active", HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
	})

	s := NewHealthCheckService(repo, client.NewHealthClient(5*time.Second))
	s.now = func() time.Time { return now }

	results, err := s.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(results) != 2 {
		t.Fatalf("len = %d, want 2", len(results))
	}

	byID := map[int64]string{}
	for _, r := range results {
		byID[r.ServerID] = r.HealthStatus
	}
	if byID[1] != "online" {
		t.Errorf("server 1 = %s, want online", byID[1])
	}
	if byID[2] != "degraded" {
		t.Errorf("server 2 = %s, want degraded", byID[2])
	}
}

func TestStartBackground(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := repository.NewMemoryServerRepository()
	now := time.Now()
	_ = repo.Create(context.Background(), &model.MCPServer{
		ID: 1, Name: "s1", Endpoint: srv.URL, Version: "1.0.0",
		Status: "active", HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
	})

	s := NewHealthCheckService(repo, client.NewHealthClient(5*time.Second))
	s.now = func() time.Time { return now }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// logf 传 nil，顺便验证空日志兜底不会 panic。
	s.StartBackground(ctx, 20*time.Millisecond, nil)

	// 期望：启动立即检查 1 次，之后每个 tick 1 次，2 秒内至少 3 次。
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for background checks, calls = %d", calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
