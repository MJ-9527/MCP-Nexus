package service

import (
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"context"
	"errors"
)

var ErrInvalidAdaptation = errors.New("invalid adaptation task")

type AdaptationService struct {
	repo  repository.AdaptationTaskRepository
	tools repository.ToolRepository
}

func NewAdaptationService(r repository.AdaptationTaskRepository, t repository.ToolRepository) *AdaptationService {
	return &AdaptationService{repo: r, tools: t}
}
func (s *AdaptationService) Create(ctx context.Context, t *model.ToolAdaptationTask) error {
	if t == nil || t.ToolID <= 0 || t.Status == "" || t.TaskType == "" {
		return ErrInvalidAdaptation
	}
	if _, e := s.tools.FindByID(ctx, t.ToolID); e != nil {
		return e
	}
	return s.repo.Create(ctx, t)
}
func (s *AdaptationService) Get(ctx context.Context, id int64) (*model.ToolAdaptationTask, error) {
	if id <= 0 {
		return nil, ErrInvalidAdaptation
	}
	return s.repo.FindByID(ctx, id)
}
func (s *AdaptationService) List(ctx context.Context, id int64) ([]*model.ToolAdaptationTask, error) {
	if id <= 0 {
		return nil, ErrInvalidAdaptation
	}
	return s.repo.ListByTool(ctx, id)
}

func (s *AdaptationService) UpdateStatus(ctx context.Context, id int64, status, errorMessage string) error {
	if id <= 0 {
		return ErrInvalidAdaptation
	}
	switch status {
	case "pending", "running", "success", "failed":
	default:
		return ErrInvalidAdaptation
	}
	return s.repo.UpdateStatus(ctx, id, status, errorMessage)
}
