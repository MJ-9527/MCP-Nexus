package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %s, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHealthClient(5 * time.Second)
	res := c.Check(context.Background(), srv.URL)
	if res.Err != nil {
		t.Fatalf("err = %v, want nil", res.Err)
	}
	if res.Latency < 0 {
		t.Errorf("latency < 0: %v", res.Latency)
	}
}

func TestCheckTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %s, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHealthClient(5 * time.Second)
	res := c.Check(context.Background(), srv.URL+"/")
	if res.Err != nil {
		t.Fatalf("err = %v, want nil", res.Err)
	}
}

func TestCheckDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHealthClient(5 * time.Second)
	res := c.Check(context.Background(), srv.URL)
	if !errors.Is(res.Err, ErrUnhealthy) {
		t.Fatalf("err = %v, want ErrUnhealthy", res.Err)
	}
}

func TestCheckOffline(t *testing.T) {
	c := NewHealthClient(2 * time.Second)
	// 端口 1 通常无服务监听，会立即返回 connection refused
	res := c.Check(context.Background(), "http://127.0.0.1:1")
	if res.Err == nil {
		t.Fatal("err = nil, want connection error")
	}
}

func TestCheckTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := NewHealthClient(50 * time.Millisecond)
	res := c.Check(context.Background(), srv.URL)
	if res.Err == nil {
		t.Fatal("err = nil, want timeout error")
	}
}
