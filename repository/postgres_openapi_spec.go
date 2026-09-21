package repository

import (
	"context"
	"errors"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresOpenAPISpecRepository Postgres 实现 OpenAPISpecRepository。
type PostgresOpenAPISpecRepository struct{ pool *pgxpool.Pool }

func NewPostgresOpenAPISpecRepository(pool *pgxpool.Pool) *PostgresOpenAPISpecRepository {
	return &PostgresOpenAPISpecRepository{pool: pool}
}

// 编译期接口校验。
var _ OpenAPISpecRepository = (*PostgresOpenAPISpecRepository)(nil)

// Save UPSERT by tool_id：重复导入时更新 method/path/params 而非冲突报错（幂等导入）。
func (r *PostgresOpenAPISpecRepository) Save(ctx context.Context, spec *model.OpenAPISpec) error {
	const query = `INSERT INTO mcp_tool_openapi_specs (tool_id, method, path_template, params)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tool_id) DO UPDATE SET
			method = EXCLUDED.method,
			path_template = EXCLUDED.path_template,
			params = EXCLUDED.params,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, query, spec.ToolID, spec.Method, spec.PathTemplate, spec.Params).
		Scan(&spec.ID, &spec.CreatedAt, &spec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (r *PostgresOpenAPISpecRepository) FindByToolID(ctx context.Context, toolID int64) (*model.OpenAPISpec, error) {
	const query = `SELECT id, tool_id, method, path_template, params, created_at, updated_at
		FROM mcp_tool_openapi_specs WHERE tool_id = $1`
	return scanOpenAPISpec(r.pool.QueryRow(ctx, query, toolID))
}

func (r *PostgresOpenAPISpecRepository) ListByServerID(ctx context.Context, serverID int64) ([]*model.OpenAPISpec, error) {
	const query = `SELECT s.id, s.tool_id, s.method, s.path_template, s.params, s.created_at, s.updated_at
		FROM mcp_tool_openapi_specs s
		JOIN mcp_tools t ON t.id = s.tool_id
		WHERE t.server_id = $1
		ORDER BY s.id`
	rows, err := r.pool.Query(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*model.OpenAPISpec, 0)
	for rows.Next() {
		spec, err := scanOpenAPISpec(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, spec)
	}
	return result, rows.Err()
}

type openapiSpecRowScanner interface{ Scan(...any) error }

func scanOpenAPISpec(row openapiSpecRowScanner) (*model.OpenAPISpec, error) {
	spec := new(model.OpenAPISpec)
	err := row.Scan(&spec.ID, &spec.ToolID, &spec.Method, &spec.PathTemplate, &spec.Params, &spec.CreatedAt, &spec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return spec, nil
}
