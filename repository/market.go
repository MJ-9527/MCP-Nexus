package repository

import (
	"MCP-Nexus/model"
	"context"
)

type CategoryRepository interface {
	Create(ctx context.Context, category *model.ToolCategory) error
	FindByID(ctx context.Context, id int64) (*model.ToolCategory, error)
	FindBySlug(ctx context.Context, slug string) (*model.ToolCategory, error)
	List(ctx context.Context) ([]*model.ToolCategory, error)
}

type ToolVersionRepository interface {
	Create(ctx context.Context, version *model.ToolVersion) error
	FindByTool(ctx context.Context, toolID int64) ([]*model.ToolVersion, error)
	FindCurrent(ctx context.Context, toolID int64) (*model.ToolVersion, error)
	SetCurrent(ctx context.Context, toolID, versionID int64) error
}

type ToolRatingRepository interface {
	Create(ctx context.Context, rating *model.ToolRating) error
	FindByTool(ctx context.Context, toolID int64, limit, offset int) ([]*model.ToolRating, error)
	AggregateByTool(ctx context.Context, toolID int64) (*model.ToolRatingSummary, error)
}
