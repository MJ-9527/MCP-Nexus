package repository

import (
	"context"
	"errors"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresReviewRepository struct{ pool *pgxpool.Pool }

func NewPostgresReviewRepository(pool *pgxpool.Pool) *PostgresReviewRepository {
	return &PostgresReviewRepository{pool: pool}
}

// Upsert 创建或更新评论（UNIQUE(tool_id, user_id) 冲突时更新）
func (r *PostgresReviewRepository) Upsert(ctx context.Context, review *model.Review) error {
	const query = `
		INSERT INTO reviews (tool_id, user_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tool_id, user_id)
		DO UPDATE SET rating = EXCLUDED.rating, comment = EXCLUDED.comment, updated_at = NOW()
		RETURNING id, created_at, updated_at`
	return r.pool.QueryRow(ctx, query, review.ToolID, review.UserID, review.Rating, review.Comment).
		Scan(&review.ID, &review.CreatedAt, &review.UpdatedAt)
}

func (r *PostgresReviewRepository) ListByTool(ctx context.Context, toolID int64, limit int) ([]*model.Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	const query = `
		SELECT rv.id, rv.tool_id, rv.user_id, COALESCE(u.username, ''), rv.rating, rv.comment, rv.created_at, rv.updated_at
		FROM reviews rv LEFT JOIN users u ON u.id = rv.user_id
		WHERE rv.tool_id = $1 ORDER BY rv.created_at DESC LIMIT $2`
	rows, err := r.pool.Query(ctx, query, toolID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reviews := make([]*model.Review, 0)
	for rows.Next() {
		rv := new(model.Review)
		if err := rows.Scan(&rv.ID, &rv.ToolID, &rv.UserID, &rv.Username, &rv.Rating, &rv.Comment, &rv.CreatedAt, &rv.UpdatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, rv)
	}
	return reviews, rows.Err()
}

func (r *PostgresReviewRepository) StatsByTool(ctx context.Context, toolID int64) (*ReviewStats, error) {
	const query = `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM reviews WHERE tool_id = $1`
	stats := new(ReviewStats)
	if err := r.pool.QueryRow(ctx, query, toolID).Scan(&stats.AvgRating, &stats.Count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ReviewStats{}, nil
		}
		return nil, err
	}
	return stats, nil
}
