package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidRating = errors.New("invalid rating")
	ErrInvalidReview = errors.New("invalid review")
	ErrRatingExists  = repository.ErrConflict
)

type ToolRatingService struct {
	ratings repository.ToolRatingRepository
	tools   repository.ToolRepository
}

func NewToolRatingService(ratings repository.ToolRatingRepository, tools repository.ToolRepository) *ToolRatingService {
	return &ToolRatingService{ratings: ratings, tools: tools}
}

func (s *ToolRatingService) Create(ctx context.Context, rating *model.ToolRating) error {
	if rating == nil || rating.ToolID <= 0 || rating.UserID <= 0 || rating.Rating < 1 || rating.Rating > 5 {
		return ErrInvalidRating
	}
	if utf8.RuneCountInString(rating.Comment) > 500 {
		return ErrInvalidReview
	}
	// 评论不允许携带常见凭证/密钥字段，避免把敏感信息写入市场数据。
	lower := strings.ToLower(rating.Comment)
	for _, word := range []string{"password", "token", "secret", "api_key", "apikey", "authorization"} {
		if strings.Contains(lower, word) {
			return ErrInvalidReview
		}
	}
	if _, err := s.tools.FindByID(ctx, rating.ToolID); err != nil {
		return err
	}
	return s.ratings.Create(ctx, rating)
}

func (s *ToolRatingService) List(ctx context.Context, toolID int64, page, pageSize int) ([]*model.ToolRating, int64, error) {
	if toolID <= 0 || page <= 0 || pageSize <= 0 || pageSize > 100 {
		return nil, 0, ErrInvalidRating
	}
	items, total, err := s.ratings.FindByTool(ctx, toolID, pageSize, (page-1)*pageSize)
	return items, total, err
}

func (s *ToolRatingService) Summary(ctx context.Context, toolID int64) (*model.ToolRatingSummary, error) {
	if toolID <= 0 {
		return nil, ErrInvalidRating
	}
	return s.ratings.AggregateByTool(ctx, toolID)
}
