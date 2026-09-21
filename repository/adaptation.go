package repository

import (
	"MCP-Nexus/model"
	"context"
)

type AdaptationTaskRepository interface {
	Create(context.Context, *model.ToolAdaptationTask) error
	FindByID(context.Context, int64) (*model.ToolAdaptationTask, error)
	ListByTool(context.Context, int64) ([]*model.ToolAdaptationTask, error)
	UpdateStatus(context.Context, int64, string, string) error
}
