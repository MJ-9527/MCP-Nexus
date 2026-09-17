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

const toolColumns = `id, server_id, category_id, name, description, category, tags, input_schema,
	version, published, is_sensitive, sensitive_level, health_status, call_count,
	average_rating, rating_count, created_at, updated_at`

type PostgresToolRepository struct{ pool *pgxpool.Pool }

func NewPostgresToolRepository(pool *pgxpool.Pool) *PostgresToolRepository {
	return &PostgresToolRepository{pool: pool}
}

func (r *PostgresToolRepository) Create(ctx context.Context, tool *model.MCPTool) error {
	const query = `INSERT INTO mcp_tools (server_id, category_id, name, description, category, tags, input_schema, version, published, health_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, call_count, average_rating, rating_count, created_at, updated_at`
	err := r.pool.QueryRow(ctx, query, tool.ServerID, tool.CategoryID, tool.Name, tool.Description, tool.Category, tool.Tags, tool.InputSchema, tool.Version, tool.Published, tool.HealthStatus).
		Scan(&tool.ID, &tool.CallCount, &tool.AverageRating, &tool.RatingCount, &tool.CreatedAt, &tool.UpdatedAt)
	return mapToolError(err)
}

func (r *PostgresToolRepository) FindByID(ctx context.Context, id int64) (*model.MCPTool, error) {
	return r.findOne(ctx, `SELECT `+toolColumns+` FROM mcp_tools WHERE id = $1`, id)
}

func (r *PostgresToolRepository) FindByName(ctx context.Context, serverID int64, name string) (*model.MCPTool, error) {
	return r.findOne(ctx, `SELECT `+toolColumns+` FROM mcp_tools WHERE server_id = $1 AND name = $2`, serverID, name)
}

func (r *PostgresToolRepository) FindPublishedByName(ctx context.Context, name string) (*model.MCPTool, error) {
	return r.findOne(ctx, `SELECT `+toolColumns+` FROM mcp_tools WHERE name = $1 AND published = true ORDER BY id LIMIT 1`, name)
}

func (r *PostgresToolRepository) List(ctx context.Context, filter ToolFilter) ([]*model.MCPTool, error) {
	query := `SELECT ` + toolColumns + ` FROM mcp_tools`
	conditions, args := make([]string, 0), make([]any, 0)
	if filter.ServerID != nil {
		args = append(args, *filter.ServerID)
		conditions = append(conditions, "server_id = $"+strconv.Itoa(len(args)))
	}
	if filter.Name != "" {
		args = append(args, "%"+filter.Name+"%")
		conditions = append(conditions, "name ILIKE $"+strconv.Itoa(len(args)))
	}
	if filter.Keyword != "" {
		args = append(args, "%"+filter.Keyword+"%")
		conditions = append(conditions, "(name ILIKE $"+strconv.Itoa(len(args))+" OR description ILIKE $"+strconv.Itoa(len(args))+")")
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		conditions = append(conditions, "(category = $"+strconv.Itoa(len(args))+" OR category_id IN (SELECT id FROM tool_categories WHERE slug = $"+strconv.Itoa(len(args))+" OR name = $"+strconv.Itoa(len(args))+"))")
	}
	if len(filter.Tags) > 0 {
		args = append(args, filter.Tags)
		conditions = append(conditions, "tags @> $"+strconv.Itoa(len(args)))
	}
	if filter.Published != nil {
		args = append(args, *filter.Published)
		conditions = append(conditions, "published = $"+strconv.Itoa(len(args)))
	}
	if filter.HealthStatus != "" {
		args = append(args, filter.HealthStatus)
		conditions = append(conditions, "health_status = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int64
	countQuery := "SELECT COUNT(*) FROM mcp_tools"
	if len(conditions) > 0 {
		countQuery += " WHERE " + strings.Join(conditions, " AND ")
	}
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query += " " + toolOrderBy(filter.Sort)
	if filter.PageSize > 0 {
		args = append(args, filter.PageSize)
		query += " LIMIT $" + strconv.Itoa(len(args))
		if filter.Page > 0 {
			args = append(args, (filter.Page-1)*filter.PageSize)
			query += " OFFSET $" + strconv.Itoa(len(args))
		}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	tools := make([]*model.MCPTool, 0)
	for rows.Next() {
		tool, err := scanTool(rows)
		if err != nil {
			return nil, 0, err
		}
		tools = append(tools, tool)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return tools, total, nil
}

func toolOrderBy(sort string) string {
	switch sort {
	case "rating":
		return "ORDER BY average_rating DESC, rating_count DESC, id DESC"
	case "name":
		return "ORDER BY name ASC, id ASC"
	case "newest":
		return "ORDER BY created_at DESC, id DESC"
	case "popularity", "":
		return "ORDER BY call_count DESC, id DESC"
	default:
		return "ORDER BY call_count DESC, id DESC"
	}
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

func (r *PostgresToolRepository) UpdateSensitivity(ctx context.Context, id int64, sensitive bool, level *string) error {
	result, err := r.pool.Exec(ctx, `UPDATE mcp_tools SET is_sensitive=$1, sensitive_level=$2, updated_at=NOW() WHERE id=$3`, sensitive, level, id)
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
	err := row.Scan(&tool.ID, &tool.ServerID, &tool.CategoryID, &tool.Name, &tool.Description, &tool.Category, &tool.Tags, &tool.InputSchema, &tool.Version, &tool.Published, &tool.IsSensitive, &tool.SensitiveLevel, &tool.HealthStatus, &tool.CallCount, &tool.AverageRating, &tool.RatingCount, &tool.CreatedAt, &tool.UpdatedAt)
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
