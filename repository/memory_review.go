package repository

import (
	"context"
	"sort"
	"sync"

	"MCP-Nexus/model"
)

// MemoryReviewRepository 内存版评论仓库（单测/本地开发用）
type MemoryReviewRepository struct {
	mu      sync.RWMutex
	nextID  int64
	reviews map[int64]*model.Review // key: review id
}

func NewMemoryReviewRepository() *MemoryReviewRepository {
	return &MemoryReviewRepository{reviews: make(map[int64]*model.Review)}
}

func (r *MemoryReviewRepository) Upsert(ctx context.Context, review *model.Review) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rv := range r.reviews {
		if rv.ToolID == review.ToolID && rv.UserID == review.UserID {
			rv.Rating = review.Rating
			rv.Comment = review.Comment
			review.ID = rv.ID
			return nil
		}
	}
	r.nextID++
	review.ID = r.nextID
	r.reviews[review.ID] = review
	return nil
}

func (r *MemoryReviewRepository) ListByTool(ctx context.Context, toolID int64, limit int) ([]*model.Review, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	out := make([]*model.Review, 0)
	for _, rv := range r.reviews {
		if rv.ToolID == toolID {
			out = append(out, rv)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *MemoryReviewRepository) StatsByTool(ctx context.Context, toolID int64) (*ReviewStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := &ReviewStats{}
	var total int64
	for _, rv := range r.reviews {
		if rv.ToolID == toolID {
			total += int64(rv.Rating)
			stats.Count++
		}
	}
	if stats.Count > 0 {
		stats.AvgRating = float64(total) / float64(stats.Count)
	}
	return stats, nil
}
