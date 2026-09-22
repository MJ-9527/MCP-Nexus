package repository

import (
	"MCP-Nexus/model"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAdaptationTaskRepository struct{ pool *pgxpool.Pool }

func NewPostgresAdaptationTaskRepository(pool *pgxpool.Pool) *PostgresAdaptationTaskRepository {
	return &PostgresAdaptationTaskRepository{pool: pool}
}

const adaptationColumns = `id,tool_id,task_type,status,source_url,error_message,created_at,updated_at`

func scanAdaptation(row interface{ Scan(...any) error }) (*model.ToolAdaptationTask, error) {
	t := new(model.ToolAdaptationTask)
	err := row.Scan(&t.ID, &t.ToolID, &t.TaskType, &t.Status, &t.SourceURL, &t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}
func (r *PostgresAdaptationTaskRepository) Create(ctx context.Context, t *model.ToolAdaptationTask) error {
	return r.pool.QueryRow(ctx, `INSERT INTO tool_adaptation_tasks (tool_id,task_type,status,source_url,error_message) VALUES ($1,$2,$3,$4,$5) RETURNING `+adaptationColumns, t.ToolID, t.TaskType, t.Status, t.SourceURL, t.ErrorMessage).Scan(&t.ID, &t.ToolID, &t.TaskType, &t.Status, &t.SourceURL, &t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt)
}
func (r *PostgresAdaptationTaskRepository) FindByID(ctx context.Context, id int64) (*model.ToolAdaptationTask, error) {
	return scanAdaptation(r.pool.QueryRow(ctx, `SELECT `+adaptationColumns+` FROM tool_adaptation_tasks WHERE id=$1`, id))
}
func (r *PostgresAdaptationTaskRepository) ListByTool(ctx context.Context, id int64) ([]*model.ToolAdaptationTask, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+adaptationColumns+` FROM tool_adaptation_tasks WHERE tool_id=$1 ORDER BY created_at DESC,id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.ToolAdaptationTask, 0)
	for rows.Next() {
		t, e := scanAdaptation(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PostgresAdaptationTaskRepository) UpdateStatus(ctx context.Context, id int64, status, errorMessage string) error {
	result, err := r.pool.Exec(ctx, `UPDATE tool_adaptation_tasks SET status=$1,error_message=$2,updated_at=NOW() WHERE id=$3`, status, errorMessage, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
