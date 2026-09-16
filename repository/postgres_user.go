package repository

import (
	"context"
	"errors"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUserRepository struct{ pool *pgxpool.Pool }

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *model.User) error {
	const query = `INSERT INTO users (username, password_hash, status)
		VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, query, user.Username, user.PasswordHash, user.Status).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	return mapUserError(err)
}

func (r *PostgresUserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	const query = `SELECT u.id, u.username, u.password_hash, u.status, u.created_at, u.updated_at,
		COALESCE(array_agg(r.name ORDER BY r.name) FILTER (WHERE r.name IS NOT NULL), '{}')
		FROM users u LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.username = $1
		GROUP BY u.id`
	user := new(model.User)
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Status,
		&user.CreatedAt, &user.UpdatedAt, &user.Roles,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(user.Roles) > 0 {
		user.Role = user.Roles[0]
	}
	return user, nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `SELECT id, username, password_hash, status, created_at, updated_at FROM users WHERE id = $1`
	user := new(model.User)
	err := r.pool.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

func (r *PostgresUserRepository) CreateRole(ctx context.Context, role *model.Role) error {
	const query = `INSERT INTO roles (name, description) VALUES ($1, $2)
		RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, query, role.Name, role.Description).Scan(&role.ID, &role.CreatedAt)
	return mapUserError(err)
}

func (r *PostgresUserRepository) FindRoleByID(ctx context.Context, id int64) (*model.Role, error) {
	const query = `SELECT id, name, description, created_at FROM roles WHERE id = $1`
	role := new(model.Role)
	err := r.pool.QueryRow(ctx, query, id).Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return role, err
}

func (r *PostgresUserRepository) AssignRole(ctx context.Context, userID, roleID int64) error {
	const query = `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`
	_, err := r.pool.Exec(ctx, query, userID, roleID)
	return mapUserError(err)
}

func mapUserError(err error) error {
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
