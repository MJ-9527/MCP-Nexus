package repository

import (
	"context"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ToolVersionRepository 工具版本历史数据访问接口
type ToolVersionRepository interface {
	// CreateVersion 记录工具版本历史
	CreateVersion(ctx context.Context, v *model.ToolVersion) error
	// ListVersions 查询工具版本历史
	ListVersions(ctx context.Context, toolID int64) ([]model.ToolVersion, error)
}

type PostgresToolVersionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresToolVersionRepository(pool *pgxpool.Pool) *PostgresToolVersionRepository {
	return &PostgresToolVersionRepository{pool: pool}
}

func (r *PostgresToolVersionRepository) CreateVersion(ctx context.Context, v *model.ToolVersion) error {
	const query = `INSERT INTO tool_versions (tool_id, version, input_schema, changelog, status)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, v.ToolID, v.Version, v.InputSchema, v.Changelog, v.Status).
		Scan(&v.ID, &v.CreatedAt)
}

func (r *PostgresToolVersionRepository) ListVersions(ctx context.Context, toolID int64) ([]model.ToolVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tool_id, version, input_schema, changelog, status, created_at
		FROM tool_versions WHERE tool_id = $1 ORDER BY created_at DESC`, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make([]model.ToolVersion, 0)
	for rows.Next() {
		var v model.ToolVersion
		if err := rows.Scan(&v.ID, &v.ToolID, &v.Version, &v.InputSchema, &v.Changelog, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}
