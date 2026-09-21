package repository

import (
	"context"
	"time"
)

type AuditRetentionPreview struct {
	ArchiveBefore time.Time `json:"archive_before"`
	DeleteBefore  time.Time `json:"delete_before"`
	ArchiveRows   int64     `json:"archive_rows"`
	DeleteRows    int64     `json:"delete_rows"`
}

type AuditRetentionResult struct {
	ArchivedRows int64 `json:"archived_rows"`
	DeletedRows  int64 `json:"deleted_rows"`
}

type AuditRetentionRepository interface {
	Preview(context.Context, time.Time, time.Time) (*AuditRetentionPreview, error)
	ArchiveBatch(context.Context, time.Time, int) (int64, error)
	DeleteArchiveBatch(context.Context, time.Time, int) (int64, error)
}
