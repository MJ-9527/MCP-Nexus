package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const toolColumns = `id, server_id, name, description, category, tags, input_schema,
	version, published, health_status, call_count, created_at, updated_at`

type PostgresToolRepository struct{ pool *pgxpool.Pool }

func NewPostgresToolRepository(pool *pgxpool.Pool) *PostgresToolRepository {
	return &PostgresToolRepository{pool: pool}
}

func (r *PostgresToolRepository) Create(ctx context.Context, tool *model.MCPTool) error {
	const query = `INSERT INTO mcp_tools (server_id, name, description, category, tags, input_schema, version, published, health_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, call_count, created_at, updated_at`
	// tags 列 NOT NULL，nil 切片写空数组
	tags := tool.Tags
	if tags == nil {
		tags = []string{}
	}
	err := r.pool.QueryRow(ctx, query, tool.ServerID, tool.Name, tool.Description, tool.Category, tags, tool.InputSchema, tool.Version, tool.Published, tool.HealthStatus).Scan(&tool.ID, &tool.CallCount, &tool.CreatedAt, &tool.UpdatedAt)
	return mapToolError(err)
}

func (r *PostgresToolRepository) FindByID(ctx context.Context, id int64) (*model.MCPTool, error) {
	return r.findOne(ctx, `SELECT `+toolColumns+` FROM mcp_tools WHERE id = $1`, id)
}

func (r *PostgresToolRepository) FindByName(ctx context.Context, serverID int64, name string) (*model.MCPTool, error) {
	return r.findOne(ctx, `SELECT `+toolColumns+` FROM mcp_tools WHERE server_id = $1 AND name = $2`, serverID, name)
}

// FindPublishedByName 跨 Server 按名称精确查找第一个已发布工具，供网关调用使用。
func (r *PostgresToolRepository) FindPublishedByName(ctx context.Context, name string) (*model.MCPTool, error) {
	return r.findOne(ctx, `SELECT `+toolColumns+` FROM mcp_tools WHERE name = $1 AND published = TRUE ORDER BY id LIMIT 1`, name)
}

// buildToolWhere 组装过滤条件（Keyword 同时匹配 name/description）
func buildToolWhere(filter ToolFilter) (string, []any) {
	conditions, args := make([]string, 0), make([]any, 0)
	if filter.ServerID != nil {
		args = append(args, *filter.ServerID)
		conditions = append(conditions, "server_id = $"+strconv.Itoa(len(args)))
	}
	if filter.Name != "" {
		args = append(args, "%"+filter.Name+"%")
		conditions = append(conditions, "name ILIKE $"+strconv.Itoa(len(args)))
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		conditions = append(conditions, "category = $"+strconv.Itoa(len(args)))
	}
	if filter.Published != nil {
		args = append(args, *filter.Published)
		conditions = append(conditions, "published = $"+strconv.Itoa(len(args)))
	}
	if filter.HealthStatus != "" {
		args = append(args, filter.HealthStatus)
		conditions = append(conditions, "health_status = $"+strconv.Itoa(len(args)))
	}
	if filter.ExcludeOffline {
		conditions = append(conditions, "health_status != 'offline'")
	}
	if filter.Keyword != "" {
		args = append(args, "%"+filter.Keyword+"%")
		n := strconv.Itoa(len(args))
		conditions = append(conditions, "(name ILIKE $"+n+" OR description ILIKE $"+n+")")
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (r *PostgresToolRepository) List(ctx context.Context, filter ToolFilter) ([]*model.MCPTool, error) {
	where, args := buildToolWhere(filter)
	query := `SELECT ` + toolColumns + ` FROM mcp_tools` + where
	if filter.PageSize > 0 {
		query += " ORDER BY id LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
		args = append(args, filter.PageSize, filter.Offset())
	} else {
		query += " ORDER BY id"
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tools := make([]*model.MCPTool, 0)
	for rows.Next() {
		tool, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tools, nil
}

func (r *PostgresToolRepository) Count(ctx context.Context, filter ToolFilter) (int64, error) {
	where, args := buildToolWhere(filter)
	var total int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM mcp_tools"+where, args...).Scan(&total)
	return total, err
}

func (r *PostgresToolRepository) UpdatePublished(ctx context.Context, id int64, published bool) error {
	const query = `UPDATE mcp_tools SET published = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.pool.Exec(ctx, query, published, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresToolRepository) findOne(ctx context.Context, query string, args ...any) (*model.MCPTool, error) {
	return scanTool(r.pool.QueryRow(ctx, query, args...))
}

type toolRowScanner interface{ Scan(...any) error }

func scanTool(row toolRowScanner) (*model.MCPTool, error) {
	tool := new(model.MCPTool)
	err := row.Scan(&tool.ID, &tool.ServerID, &tool.Name, &tool.Description, &tool.Category, &tool.Tags, &tool.InputSchema, &tool.Version, &tool.Published, &tool.HealthStatus, &tool.CallCount, &tool.CreatedAt, &tool.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return tool, nil
}
func mapToolError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
