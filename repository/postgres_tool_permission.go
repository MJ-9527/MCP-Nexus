package repository

import (
	"context"
	"errors"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresToolPermissionRepository struct{ pool *pgxpool.Pool }

func NewPostgresToolPermissionRepository(pool *pgxpool.Pool) *PostgresToolPermissionRepository {
	return &PostgresToolPermissionRepository{pool: pool}
}

func (r *PostgresToolPermissionRepository) Grant(ctx context.Context, permission *model.ToolPermission) error {
	const query = `INSERT INTO tool_permissions (tool_id, user_id, role_id, action)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at`
	if permission == nil || permission.ToolID <= 0 || permission.Action == "" ||
		(permission.UserID == nil && permission.RoleID == nil) ||
		(permission.UserID != nil && permission.RoleID != nil) {
		return errors.New("invalid tool permission")
	}
	err := r.pool.QueryRow(ctx, query, permission.ToolID, permission.UserID, permission.RoleID, permission.Action).
		Scan(&permission.ID, &permission.CreatedAt)
	return mapPermissionError(err)
}

func (r *PostgresToolPermissionRepository) Revoke(ctx context.Context, toolID int64, userID, roleID *int64, action string) error {
	if toolID <= 0 || action == "" || (userID == nil && roleID == nil) || (userID != nil && roleID != nil) {
		return errors.New("invalid tool permission")
	}
	const query = `DELETE FROM tool_permissions
		WHERE tool_id = $1 AND action = $2 AND user_id IS NOT DISTINCT FROM $3 AND role_id IS NOT DISTINCT FROM $4`
	result, err := r.pool.Exec(ctx, query, toolID, action, userID, roleID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresToolPermissionRepository) ListByTool(ctx context.Context, toolID int64) ([]*model.ToolPermission, error) {
	if toolID <= 0 {
		return nil, errors.New("invalid tool ID")
	}
	const query = `SELECT id, tool_id, user_id, role_id, action, created_at
		FROM tool_permissions WHERE tool_id = $1 ORDER BY id`
	rows, err := r.pool.Query(ctx, query, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	permissions := make([]*model.ToolPermission, 0)
	for rows.Next() {
		permission := new(model.ToolPermission)
		if err := rows.Scan(&permission.ID, &permission.ToolID, &permission.UserID, &permission.RoleID, &permission.Action, &permission.CreatedAt); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}

func (r *PostgresToolPermissionRepository) HasPermission(ctx context.Context, userID, toolID int64, action string) (bool, error) {
	if userID <= 0 || toolID <= 0 || action == "" {
		return false, errors.New("invalid permission query")
	}
	const query = `SELECT EXISTS (
		SELECT 1 FROM tool_permissions p
		WHERE p.tool_id = $1 AND p.action = $2 AND p.user_id = $3
		UNION ALL
		SELECT 1 FROM tool_permissions p
		JOIN user_roles ur ON ur.role_id = p.role_id
		WHERE p.tool_id = $1 AND p.action = $2 AND ur.user_id = $3
	)`
	var allowed bool
	if err := r.pool.QueryRow(ctx, query, toolID, action, userID).Scan(&allowed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return allowed, nil
}

func (r *PostgresToolPermissionRepository) ListToolNamesByRole(ctx context.Context, roleName string, action string) ([]string, error) {
	if roleName == "" || action == "" {
		return nil, errors.New("invalid permission query")
	}
	const query = `SELECT DISTINCT t.name
		FROM tool_permissions p
		JOIN roles r ON r.id = p.role_id
		JOIN mcp_tools t ON t.id = p.tool_id
		WHERE r.name = $1 AND p.action = $2 AND t.published = true
		ORDER BY t.name`
	rows, err := r.pool.Query(ctx, query, roleName, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (r *PostgresToolPermissionRepository) ListToolNamesByUser(ctx context.Context, userID int64, action string) ([]string, error) {
	if userID <= 0 || action == "" {
		return nil, errors.New("invalid permission query")
	}
	const query = `SELECT DISTINCT t.name
		FROM tool_permissions p
		JOIN mcp_tools t ON t.id = p.tool_id
		WHERE p.user_id = $1 AND p.action = $2 AND t.published = true
		ORDER BY t.name`
	rows, err := r.pool.Query(ctx, query, userID, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
func (r *PostgresToolPermissionRepository) ConfigureRolePermissions(ctx context.Context, toolID int64, sensitive bool, level *string, permissions []*model.ToolPermission) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE mcp_tools SET is_sensitive=$1, sensitive_level=$2, updated_at=NOW() WHERE id=$3`, sensitive, level, toolID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err = tx.Exec(ctx, `DELETE FROM tool_permissions WHERE tool_id=$1 AND role_id IS NOT NULL`, toolID); err != nil {
		return err
	}
	for _, p := range permissions {
		if p == nil || p.RoleID == nil || p.ToolID != toolID || p.Action == "" {
			return errors.New("invalid tool permission")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO tool_permissions(tool_id,role_id,action) VALUES($1,$2,$3)`, toolID, p.RoleID, p.Action); err != nil {
			return mapPermissionError(err)
		}
	}
	return tx.Commit(ctx)
}

func mapPermissionError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrNotFound
		}
	}
	return err
}
