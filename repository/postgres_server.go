package repository

import (
	"context"
	"errors"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const serverColumns = `
	id, name, description, endpoint, version, owner_id, status,
	health_status, last_health_check_at, created_at, updated_at`

type PostgresServerRepository struct{ pool *pgxpool.Pool }

func NewPostgresServerRepository(pool *pgxpool.Pool) *PostgresServerRepository {
	return &PostgresServerRepository{pool: pool}
}

func (r *PostgresServerRepository) Create(ctx context.Context, server *model.MCPServer) error {
	const query = `INSERT INTO mcp_servers (name, description, endpoint, version, owner_id, status, health_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at, updated_at`
	err := r.pool.QueryRow(ctx, query, server.Name, server.Description, server.Endpoint, server.Version, server.OwnerID, server.Status, server.HealthStatus).Scan(&server.ID, &server.CreatedAt, &server.UpdatedAt)
	return mapServerError(err)
}

func (r *PostgresServerRepository) FindByName(ctx context.Context, name string) (*model.MCPServer, error) {
	return r.findOne(ctx, `SELECT `+serverColumns+` FROM mcp_servers WHERE name = $1`, name)
}

func (r *PostgresServerRepository) FindByEndpoint(ctx context.Context, endpoint string) (*model.MCPServer, error) {
	return r.findOne(ctx, `SELECT `+serverColumns+` FROM mcp_servers WHERE endpoint = $1`, endpoint)
}

func (r *PostgresServerRepository) FindByID(ctx context.Context, id int64) (*model.MCPServer, error) {
	return r.findOne(ctx, `SELECT `+serverColumns+` FROM mcp_servers WHERE id = $1`, id)
}

func (r *PostgresServerRepository) List(ctx context.Context) ([]*model.MCPServer, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serverColumns+` FROM mcp_servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	servers := make([]*model.MCPServer, 0)
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *PostgresServerRepository) findOne(ctx context.Context, query string, arg any) (*model.MCPServer, error) {
	return scanServer(r.pool.QueryRow(ctx, query, arg))
}

type rowScanner interface{ Scan(...any) error }

func scanServer(row rowScanner) (*model.MCPServer, error) {
	server := new(model.MCPServer)
	err := row.Scan(&server.ID, &server.Name, &server.Description, &server.Endpoint, &server.Version, &server.OwnerID, &server.Status, &server.HealthStatus, &server.LastHealthCheckAt, &server.CreatedAt, &server.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return server, nil
}

func mapServerError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
