package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

type fakeAuditLogRepository struct{ log *model.AuditLog }

func (f *fakeAuditLogRepository) Create(_ context.Context, log *model.AuditLog) error {
	f.log = log
	return nil
}

func (f *fakeAuditLogRepository) BatchCreate(_ context.Context, logs []*model.AuditLog) error {
	if len(logs) > 0 {
		f.log = logs[len(logs)-1]
	}
	return nil
}

func (f *fakeAuditLogRepository) List(context.Context, repository.AuditLogFilter) ([]*model.AuditLog, int64, error) {
	if f.log == nil {
		return nil, 0, nil
	}
	return []*model.AuditLog{f.log}, 1, nil
}

func TestAuditLogServiceHashesParameters(t *testing.T) {
	repo := &fakeAuditLogRepository{}
	svc := NewAuditLogService(repo)
	params := map[string]any{"secret": "do-not-store"}

	log, err := svc.Record(context.Background(), model.CreateAuditLogRequest{
		RequestID:  "req-a8",
		Status:     "success",
		Parameters: params,
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	encoded, _ := json.Marshal(params)
	if log.ParamsDigest == "" || len(log.ParamsDigest) != 64 {
		t.Fatalf("ParamsDigest = %q, want SHA-256 hex digest", log.ParamsDigest)
	}
	if strings.Contains(log.ParamsDigest, string(encoded)) || strings.Contains(log.ParamsDigest, "do-not-store") {
		t.Fatalf("ParamsDigest contains raw parameters: %q", log.ParamsDigest)
	}
}
