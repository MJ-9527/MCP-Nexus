package repository

import (
	"context"
	"errors"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSkillsSpecRepository Postgres 实现 SkillsSpecRepository。
type PostgresSkillsSpecRepository struct{ pool *pgxpool.Pool }

func NewPostgresSkillsSpecRepository(pool *pgxpool.Pool) *PostgresSkillsSpecRepository {
	return &PostgresSkillsSpecRepository{pool: pool}
}

// 编译期接口校验。
var _ SkillsSpecRepository = (*PostgresSkillsSpecRepository)(nil)

// Save UPSERT by tool_id：重复导入时更新 endpoint/method/path/params 而非冲突报错（幂等导入）。
func (r *PostgresSkillsSpecRepository) Save(ctx context.Context, spec *model.SkillsSpec) error {
	const query = `INSERT INTO mcp_tool_skills_specs (tool_id, endpoint, method, path_template, params)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tool_id) DO UPDATE SET
			endpoint = EXCLUDED.endpoint,
			method = EXCLUDED.method,
			path_template = EXCLUDED.path_template,
			params = EXCLUDED.params,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, query,
		spec.ToolID, spec.Endpoint, spec.Method, spec.PathTemplate, spec.Params).
		Scan(&spec.ID, &spec.CreatedAt, &spec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (r *PostgresSkillsSpecRepository) FindByToolID(ctx context.Context, toolID int64) (*model.SkillsSpec, error) {
	const query = `SELECT id, tool_id, endpoint, method, path_template, params, created_at, updated_at
		FROM mcp_tool_skills_specs WHERE tool_id = $1`
	return scanSkillsSpec(r.pool.QueryRow(ctx, query, toolID))
}

func (r *PostgresSkillsSpecRepository) ListByServerID(ctx context.Context, serverID int64) ([]*model.SkillsSpec, error) {
	const query = `SELECT s.id, s.tool_id, s.endpoint, s.method, s.path_template, s.params, s.created_at, s.updated_at
		FROM mcp_tool_skills_specs s
		JOIN mcp_tools t ON t.id = s.tool_id
		WHERE t.server_id = $1
		ORDER BY s.id`
	rows, err := r.pool.Query(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*model.SkillsSpec, 0)
	for rows.Next() {
		spec, err := scanSkillsSpec(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, spec)
	}
	return result, rows.Err()
}

type skillsSpecRowScanner interface{ Scan(...any) error }

func scanSkillsSpec(row skillsSpecRowScanner) (*model.SkillsSpec, error) {
	spec := new(model.SkillsSpec)
	err := row.Scan(&spec.ID, &spec.ToolID, &spec.Endpoint, &spec.Method, &spec.PathTemplate, &spec.Params, &spec.CreatedAt, &spec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return spec, nil
}
