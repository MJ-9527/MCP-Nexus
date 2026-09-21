package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type PostgresAuditRetentionRepository struct{ pool *pgxpool.Pool }

func NewPostgresAuditRetentionRepository(pool *pgxpool.Pool) *PostgresAuditRetentionRepository {
	return &PostgresAuditRetentionRepository{pool: pool}
}

func (r *PostgresAuditRetentionRepository) Preview(ctx context.Context, archiveBefore, deleteBefore time.Time) (*AuditRetentionPreview, error) {
	v := &AuditRetentionPreview{ArchiveBefore: archiveBefore, DeleteBefore: deleteBefore}
	if e := r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE created_at < $1`, archiveBefore).Scan(&v.ArchiveRows); e != nil {
		return nil, e
	}
	if e := r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs_archive WHERE created_at < $1`, deleteBefore).Scan(&v.DeleteRows); e != nil {
		return nil, e
	}
	return v, nil
}

func (r *PostgresAuditRetentionRepository) ArchiveBatch(ctx context.Context, before time.Time, limit int) (int64, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, `WITH selected AS (SELECT * FROM audit_logs WHERE created_at < $1 ORDER BY created_at,id LIMIT $2 FOR UPDATE SKIP LOCKED), inserted AS (INSERT INTO audit_logs_archive SELECT selected.*,NOW() FROM selected ON CONFLICT(id) DO NOTHING RETURNING id) DELETE FROM audit_logs a USING selected s WHERE a.id=s.id`, before, limit)
	if e != nil {
		return 0, e
	}
	if e = tx.Commit(ctx); e != nil {
		return 0, e
	}
	return tag.RowsAffected(), nil
}

func (r *PostgresAuditRetentionRepository) DeleteArchiveBatch(ctx context.Context, before time.Time, limit int) (int64, error) {
	tag, e := r.pool.Exec(ctx, `DELETE FROM audit_logs_archive WHERE id IN (SELECT id FROM audit_logs_archive WHERE created_at < $1 ORDER BY created_at,id LIMIT $2)`, before, limit)
	if e != nil {
		return 0, e
	}
	return tag.RowsAffected(), nil
}
