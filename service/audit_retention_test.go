package service

import (
	"MCP-Nexus/repository"
	"context"
	"testing"
	"time"
)

type fakeRetentionRepo struct {
	archived bool
	deleted  bool
}

func (f *fakeRetentionRepo) Preview(context.Context, time.Time, time.Time) (*repository.AuditRetentionPreview, error) {
	return &repository.AuditRetentionPreview{ArchiveRows: 2, DeleteRows: 1}, nil
}
func (f *fakeRetentionRepo) ArchiveBatch(context.Context, time.Time, int) (int64, error) {
	if f.archived {
		return 0, nil
	}
	f.archived = true
	return 2, nil
}
func (f *fakeRetentionRepo) DeleteArchiveBatch(context.Context, time.Time, int) (int64, error) {
	if f.deleted {
		return 0, nil
	}
	f.deleted = true
	return 1, nil
}
func TestAuditRetentionRun(t *testing.T) {
	s := NewAuditRetentionService(&fakeRetentionRepo{})
	s.now = func() time.Time { return time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC) }
	v, e := s.Run(context.Background(), 90, 365, 100)
	if e != nil || v.ArchivedRows != 2 || v.DeletedRows != 1 {
		t.Fatalf("unexpected result %#v %v", v, e)
	}
}
