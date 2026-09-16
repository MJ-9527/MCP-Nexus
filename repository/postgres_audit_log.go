package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAuditLogRepository struct{ pool *pgxpool.Pool }

func NewPostgresAuditLogRepository(pool *pgxpool.Pool) *PostgresAuditLogRepository {
	return &PostgresAuditLogRepository{pool: pool}
}

func (r *PostgresAuditLogRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if log == nil {
		return errors.New("nil audit log")
	}
	const query = `INSERT INTO audit_logs
		(request_id, user_id, tool_id, duration_ms, status, denied_reason, params_digest)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, log.RequestID, log.UserID, log.ToolID, log.DurationMS,
		log.Status, log.DeniedReason, log.ParamsDigest).Scan(&log.ID, &log.CreatedAt)
}

func (r *PostgresAuditLogRepository) List(ctx context.Context, filter AuditLogFilter) ([]*model.AuditLog, int64, error) {
	args := make([]any, 0, 5)
	conditions := make([]string, 0, 4)
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.Replace(condition, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if filter.RequestID != "" {
		add("request_id = ?", filter.RequestID)
	}
	if filter.UserID != nil {
		add("user_id = ?", *filter.UserID)
	}
	if filter.ToolID != nil {
		add("tool_id = ?", *filter.ToolID)
	}
	if filter.Status != "" {
		add("status = ?", filter.Status)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	query := `SELECT id, request_id, user_id, tool_id, duration_ms, status, denied_reason, params_digest, created_at FROM audit_logs` + where +
		" ORDER BY created_at DESC, id DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs := make([]*model.AuditLog, 0)
	for rows.Next() {
		log := new(model.AuditLog)
		if err := rows.Scan(&log.ID, &log.RequestID, &log.UserID, &log.ToolID, &log.DurationMS,
			&log.Status, &log.DeniedReason, &log.ParamsDigest, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
