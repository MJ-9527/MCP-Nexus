package service

import (
	"context"
	"encoding/json"
	"fmt"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// MarketService 工具市场：浏览、搜索、详情、评分、排行、接入配置。
type MarketService struct {
	tools   repository.ToolRepository
	servers repository.ServerRepository
	reviews repository.ReviewRepository
	ch      *repository.ClickHouseAuditRepository // 可为 nil（ClickHouse 不可用时降级）
}

func NewMarketService(tools repository.ToolRepository, servers repository.ServerRepository, reviews repository.ReviewRepository, ch *repository.ClickHouseAuditRepository) *MarketService {
	return &MarketService{tools: tools, servers: servers, reviews: reviews, ch: ch}
}

// List 分类浏览 + 关键词搜索 + 分页（仅展示已发布且未下线的工具）
func (s *MarketService) List(ctx context.Context, category, keyword string, page, pageSize int) (*model.MarketListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	published := true
	filter := repository.ToolFilter{
		Category:       category,
		Keyword:        keyword,
		Published:      &published,
		ExcludeOffline: true,
		Page:           page,
		PageSize:       pageSize,
	}
	total, err := s.tools.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	tools, err := s.tools.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	items := make([]model.MarketToolItem, 0, len(tools))
	for _, t := range tools {
		item := model.MarketToolItem{
			ID: t.ID, Name: t.Name, Description: t.Description,
			Category: t.Category, Tags: t.Tags, Version: t.Version,
			HealthStatus: t.HealthStatus, CallCount: t.CallCount, CreatedAt: t.CreatedAt,
		}
		if srv, err := s.servers.FindByID(ctx, t.ServerID); err == nil {
			item.ServerName = srv.Name
		}
		if stats, err := s.reviews.StatsByTool(ctx, t.ID); err == nil {
			item.AvgRating = round1(stats.AvgRating)
			item.ReviewCount = stats.Count
		}
		items = append(items, item)
	}
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}
	return &model.MarketListResponse{
		Items: items, Page: page, PageSize: pageSize,
		Total: total, TotalPages: totalPages,
	}, nil
}

// Detail 工具详情：版本/健康/Schema + 上游 Server + 近30天调用量 + 评分统计
func (s *MarketService) Detail(ctx context.Context, id int64) (*model.MarketToolDetail, error) {
	tool, err := s.tools.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 市场详情仅展示已发布工具，未发布/草稿工具不允许通过 ID 直接访问
	if !tool.Published {
		return nil, repository.ErrNotFound
	}
	detail := &model.MarketToolDetail{
		MCPTool:   *tool,
		AvgRating: 0, ReviewCount: 0,
	}
	if srv, err := s.servers.FindByID(ctx, tool.ServerID); err == nil {
		detail.ServerName = srv.Name
		detail.ServerEndpoint = srv.Endpoint
		detail.ServerHealth = srv.HealthStatus
		detail.HealthCheckedAt = srv.LastHealthCheckAt
	}
	if s.ch != nil {
		if total, rate, err := s.ch.ToolStats30d(ctx, tool.Name); err == nil {
			detail.CallCount30d = total
			detail.SuccessRate30d = round1(rate)
		}
	}
	if stats, err := s.reviews.StatsByTool(ctx, tool.ID); err == nil {
		detail.AvgRating = round1(stats.AvgRating)
		detail.ReviewCount = stats.Count
	}
	return detail, nil
}

// UpsertReview 提交/更新评分评论
func (s *MarketService) UpsertReview(ctx context.Context, toolID, userID int64, req *model.UpsertReviewRequest) (*model.Review, error) {
	if _, err := s.tools.FindByID(ctx, toolID); err != nil {
		return nil, err
	}
	review := &model.Review{ToolID: toolID, UserID: userID, Rating: req.Rating, Comment: req.Comment}
	if err := s.reviews.Upsert(ctx, review); err != nil {
		return nil, err
	}
	return review, nil
}

// ListReviews 工具的评论列表
func (s *MarketService) ListReviews(ctx context.Context, toolID int64) ([]*model.Review, error) {
	if _, err := s.tools.FindByID(ctx, toolID); err != nil {
		return nil, err
	}
	return s.reviews.ListByTool(ctx, toolID, 50)
}

// Ranking 调用量排行（ClickHouse 聚合，不可用时返回空列表）
func (s *MarketService) Ranking(ctx context.Context, limit int) ([]model.RankingItem, error) {
	if s.ch == nil {
		return []model.RankingItem{}, nil
	}
	return s.ch.TopTools(ctx, 24*7, limit)
}

// ConfigSnippet 一键生成 MCP 接入配置片段
func (s *MarketService) ConfigSnippet(ctx context.Context, id int64, gatewayURL string) (*model.ConfigSnippetResponse, error) {
	tool, err := s.tools.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tool.HealthStatus == "offline" {
		return nil, ErrToolOffline
	}
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8080"
	}
	// 面向 MCP 客户端（Agent）的 streamable-http 接入配置
	snippet := map[string]any{
		"mcpServers": map[string]any{
			tool.Name: map[string]any{
				"url":       fmt.Sprintf("%s/mcp/tools/%s/call", gatewayURL, tool.Name),
				"transport": "http",
				"headers": map[string]string{
					"Authorization": "Bearer <你的JWT>",
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(snippet, "", "  ")
	notes := []string{
		"先通过 POST /api/auth/login 获取 JWT，替换 <你的JWT>",
		"请求体格式：{\"arguments\": {...按 input_schema 传参...}}",
		"所有调用经过网关鉴权（RBAC）与限流，调用记录写入审计日志",
	}
	return &model.ConfigSnippetResponse{ToolName: tool.Name, Snippet: raw, Notes: notes}, nil
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
