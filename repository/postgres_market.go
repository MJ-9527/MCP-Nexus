package repository

import (
	"MCP-Nexus/model"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCategoryRepository struct{ pool *pgxpool.Pool }

func NewPostgresCategoryRepository(pool *pgxpool.Pool) *PostgresCategoryRepository {
	return &PostgresCategoryRepository{pool: pool}
}

func (r *PostgresCategoryRepository) Create(ctx context.Context, category *model.ToolCategory) error {
	err := r.pool.QueryRow(ctx, `INSERT INTO tool_categories (name, slug, description) VALUES ($1,$2,$3)
		RETURNING id, created_at, updated_at`, category.Name, category.Slug, category.Description).
		Scan(&category.ID, &category.CreatedAt, &category.UpdatedAt)
	return mapMarketError(err)
}

func (r *PostgresCategoryRepository) FindByID(ctx context.Context, id int64) (*model.ToolCategory, error) {
	return r.findOne(ctx, `SELECT id,name,slug,description,created_at,updated_at FROM tool_categories WHERE id=$1`, id)
}

func (r *PostgresCategoryRepository) FindBySlug(ctx context.Context, slug string) (*model.ToolCategory, error) {
	return r.findOne(ctx, `SELECT id,name,slug,description,created_at,updated_at FROM tool_categories WHERE slug=$1`, slug)
}

func (r *PostgresCategoryRepository) findOne(ctx context.Context, query string, arg any) (*model.ToolCategory, error) {
	category := new(model.ToolCategory)
	err := r.pool.QueryRow(ctx, query, arg).Scan(&category.ID, &category.Name, &category.Slug, &category.Description, &category.CreatedAt, &category.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return category, err
}

func (r *PostgresCategoryRepository) List(ctx context.Context) ([]*model.ToolCategory, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,name,slug,description,created_at,updated_at FROM tool_categories ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*model.ToolCategory, 0)
	for rows.Next() {
		item := new(model.ToolCategory)
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type PostgresToolVersionRepository struct{ pool *pgxpool.Pool }

func NewPostgresToolVersionRepository(pool *pgxpool.Pool) *PostgresToolVersionRepository {
	return &PostgresToolVersionRepository{pool: pool}
}

const versionColumns = `id,tool_id,version,input_schema,changelog,status,is_current,released_at,created_at,updated_at`

func (r *PostgresToolVersionRepository) Create(ctx context.Context, version *model.ToolVersion) error {
	err := r.pool.QueryRow(ctx, `INSERT INTO tool_versions (tool_id,version,input_schema,changelog,status,is_current,released_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,created_at,updated_at`, version.ToolID, version.Version,
		version.InputSchema, version.Changelog, version.Status, version.IsCurrent, version.ReleasedAt).
		Scan(&version.ID, &version.CreatedAt, &version.UpdatedAt)
	return mapMarketError(err)
}

func (r *PostgresToolVersionRepository) FindByTool(ctx context.Context, toolID int64) ([]*model.ToolVersion, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+versionColumns+` FROM tool_versions WHERE tool_id=$1 ORDER BY created_at DESC,id DESC`, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*model.ToolVersion, 0)
	for rows.Next() {
		item, err := scanToolVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresToolVersionRepository) FindCurrent(ctx context.Context, toolID int64) (*model.ToolVersion, error) {
	return scanToolVersion(r.pool.QueryRow(ctx, `SELECT `+versionColumns+` FROM tool_versions WHERE tool_id=$1 AND is_current=TRUE`, toolID))
}

func (r *PostgresToolVersionRepository) SetCurrent(ctx context.Context, toolID, versionID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE tool_versions SET is_current=FALSE,updated_at=NOW() WHERE tool_id=$1 AND is_current=TRUE`, toolID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE tool_versions SET is_current=TRUE,released_at=COALESCE(released_at,NOW()),updated_at=NOW()
		WHERE id=$1 AND tool_id=$2`, versionID, toolID)
	if err != nil {
		return mapMarketError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

type versionScanner interface{ Scan(...any) error }

func scanToolVersion(row versionScanner) (*model.ToolVersion, error) {
	item := new(model.ToolVersion)
	err := row.Scan(&item.ID, &item.ToolID, &item.Version, &item.InputSchema, &item.Changelog, &item.Status, &item.IsCurrent, &item.ReleasedAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

type PostgresToolRatingRepository struct{ pool *pgxpool.Pool }

func NewPostgresToolRatingRepository(pool *pgxpool.Pool) *PostgresToolRatingRepository {
	return &PostgresToolRatingRepository{pool: pool}
}

func (r *PostgresToolRatingRepository) Create(ctx context.Context, rating *model.ToolRating) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO tool_ratings (tool_id,user_id,rating,comment) VALUES ($1,$2,$3,$4)
		RETURNING id,created_at,updated_at`, rating.ToolID, rating.UserID, rating.Rating, rating.Comment).
		Scan(&rating.ID, &rating.CreatedAt, &rating.UpdatedAt)
	if err != nil {
		return mapMarketError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE mcp_tools SET average_rating=(SELECT COALESCE(AVG(rating),0) FROM tool_ratings WHERE tool_id=$1),
		rating_count=(SELECT COUNT(*) FROM tool_ratings WHERE tool_id=$1),updated_at=NOW() WHERE id=$1`, rating.ToolID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresToolRatingRepository) FindByTool(ctx context.Context, toolID int64, limit, offset int) ([]*model.ToolRating, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tool_ratings WHERE tool_id=$1`, toolID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,tool_id,user_id,rating,comment,created_at,updated_at FROM tool_ratings
		WHERE tool_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, toolID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*model.ToolRating, 0)
	for rows.Next() {
		item := new(model.ToolRating)
		if err := rows.Scan(&item.ID, &item.ToolID, &item.UserID, &item.Rating, &item.Comment, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *PostgresToolRatingRepository) AggregateByTool(ctx context.Context, toolID int64) (*model.ToolRatingSummary, error) {
	summary := new(model.ToolRatingSummary)
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(AVG(rating),0)::DOUBLE PRECISION,COUNT(*) FROM tool_ratings WHERE tool_id=$1`, toolID).
		Scan(&summary.Average, &summary.Count)
	return summary, err
}

func mapMarketError(err error) error {
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
