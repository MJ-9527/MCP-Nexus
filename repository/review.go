package repository

import (
	"context"

	"MCP-Nexus/model"
)

// ReviewStats 工具的评分统计
type ReviewStats struct {
	AvgRating float64
	Count     int64
}

type ReviewRepository interface {
	// Upsert 创建或更新评论（每人每工具一条）
	Upsert(ctx context.Context, review *model.Review) error
	// ListByTool 按工具列出评论（最新在前）
	ListByTool(ctx context.Context, toolID int64, limit int) ([]*model.Review, error)
	// StatsByTool 评分统计（平均分、条数）
	StatsByTool(ctx context.Context, toolID int64) (*ReviewStats, error)
}
