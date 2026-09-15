package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresPermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPermissionRepository(pool *pgxpool.Pool) *PostgresPermissionRepository {
	return &PostgresPermissionRepository{pool: pool}
}

func (r *PostgresPermissionRepository) FindToolsByRole(ctx context.Context, role string) ([]string, error) {
	const query = `SELECT tool_name FROM role_tools WHERE role = $1`
	rows, err := r.pool.Query(ctx, query, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tools = append(tools, name)
	}
	return tools, rows.Err()
}

// SetPermission 授予或撤销角色对工具的调用权限。
func (r *PostgresPermissionRepository) SetPermission(ctx context.Context, role, toolName, action string) error {
	switch action {
	case "grant":
		_, err := r.pool.Exec(ctx,
			`INSERT INTO role_tools (role, tool_name) VALUES ($1, $2) ON CONFLICT DO NOTHING`, role, toolName)
		return err
	case "revoke":
		_, err := r.pool.Exec(ctx,
			`DELETE FROM role_tools WHERE role = $1 AND tool_name = $2`, role, toolName)
		return err
	default:
		return ErrInvalidParameter
	}
}
