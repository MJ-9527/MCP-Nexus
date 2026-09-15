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
