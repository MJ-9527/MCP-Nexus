package repository

import (
	"MCP-Nexus/model"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAlertRepository struct{ pool *pgxpool.Pool }

func NewPostgresAlertRepository(p *pgxpool.Pool) *PostgresAlertRepository {
	return &PostgresAlertRepository{pool: p}
}
func (r *PostgresAlertRepository) Create(ctx context.Context, a *model.AnomalyAlert) error {
	return r.pool.QueryRow(ctx, `INSERT INTO anomaly_alerts(alert_type,severity,tool_id,tool_name,message,status,triggered_at) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, a.AlertType, a.Severity, a.ToolID, a.ToolName, a.Message, a.Status, a.TriggeredAt).Scan(&a.ID)
}
func (r *PostgresAlertRepository) List(ctx context.Context, f AlertFilter) ([]*model.AnomalyAlert, int64, error) {
	where := " WHERE ($1='' OR status=$1) AND ($2='' OR severity=$2)"
	args := []any{f.Status, f.Severity}
	var total int64
	if e := r.pool.QueryRow(ctx, `SELECT count(*) FROM anomaly_alerts`+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, e := r.pool.Query(ctx, `SELECT id,alert_type,severity,tool_id,tool_name,message,status,triggered_at,acknowledged_at,acknowledged_by FROM anomaly_alerts`+where+` ORDER BY triggered_at DESC,id DESC LIMIT $3 OFFSET $4`, args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []*model.AnomalyAlert{}
	for rows.Next() {
		a := new(model.AnomalyAlert)
		if e = rows.Scan(&a.ID, &a.AlertType, &a.Severity, &a.ToolID, &a.ToolName, &a.Message, &a.Status, &a.TriggeredAt, &a.AcknowledgedAt, &a.AcknowledgedBy); e != nil {
			return nil, 0, e
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}
func (r *PostgresAlertRepository) Acknowledge(ctx context.Context, id, user int64) error {
	res, e := r.pool.Exec(ctx, `UPDATE anomaly_alerts SET status='acknowledged',acknowledged_at=NOW(),acknowledged_by=$1 WHERE id=$2 AND status='open'`, user, id)
	if e != nil {
		return e
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var _ = errors.Is
var _ = pgx.ErrNoRows
