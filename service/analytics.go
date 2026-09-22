package service

import (
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"context"
	"errors"
	"time"
)

var ErrInvalidAnalyticsQuery = errors.New("invalid analytics query")

type AnalyticsOverview struct {
	Period struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
		Days  int       `json:"days"`
	} `json:"period"`
	Summary       *model.AnalyticsSummary   `json:"summary"`
	TopTools      []model.ToolAnalyticsRank `json:"top_tools"`
	DailyTrend    []model.AnalyticsPoint    `json:"daily_trend"`
	RejectReasons []model.RejectReasonStat  `json:"reject_reasons"`
	Alerts        []*model.AnomalyAlert     `json:"alerts"`
}
type AnalyticsService struct {
	analytics repository.AnalyticsRepository
	alerts    repository.AlertRepository
	now       func() time.Time
}

func NewAnalyticsService(a repository.AnalyticsRepository, alerts repository.AlertRepository) *AnalyticsService {
	return &AnalyticsService{analytics: a, alerts: alerts, now: time.Now}
}
func (s *AnalyticsService) Overview(ctx context.Context, days int) (*AnalyticsOverview, error) {
	if s == nil || s.analytics == nil || days <= 0 || days > 365 {
		return nil, ErrInvalidAnalyticsQuery
	}
	end := s.now().UTC()
	start := end.AddDate(0, 0, -days)
	summary, e := s.analytics.Summary(ctx, start, end)
	if e != nil {
		return nil, e
	}
	top, e := s.analytics.TopTools(ctx, start, end, 10)
	if e != nil {
		return nil, e
	}
	trend, e := s.analytics.Trends(ctx, start, end, "day", nil)
	if e != nil {
		return nil, e
	}
	reasons, e := s.analytics.RejectReasons(ctx, start, end, 10)
	if e != nil {
		return nil, e
	}
	alerts, _, e := s.alerts.List(ctx, repository.AlertFilter{Status: "open", Page: 1, PageSize: 10})
	if e != nil {
		return nil, e
	}
	out := &AnalyticsOverview{Summary: summary, TopTools: top, DailyTrend: trend, RejectReasons: reasons, Alerts: alerts}
	out.Period.Start = start
	out.Period.End = end
	out.Period.Days = days
	return out, nil
}
func (s *AnalyticsService) Trends(ctx context.Context, start, end time.Time, g string, toolID *int64) ([]model.AnalyticsPoint, error) {
	if s == nil || s.analytics == nil || start.IsZero() || end.IsZero() || !start.Before(end) || (g != "hour" && g != "day") || toolID != nil && *toolID <= 0 {
		return nil, ErrInvalidAnalyticsQuery
	}
	return s.analytics.Trends(ctx, start, end, g, toolID)
}
func (s *AnalyticsService) ListAlerts(ctx context.Context, f repository.AlertFilter) ([]*model.AnomalyAlert, int64, error) {
	if f.Page <= 0 || f.PageSize <= 0 || f.PageSize > 100 {
		return nil, 0, ErrInvalidAnalyticsQuery
	}
	if f.Status != "" && f.Status != "open" && f.Status != "acknowledged" {
		return nil, 0, ErrInvalidAnalyticsQuery
	}
	return s.alerts.List(ctx, f)
}
func (s *AnalyticsService) Acknowledge(ctx context.Context, id, userID int64) error {
	if id <= 0 || userID <= 0 {
		return ErrInvalidAnalyticsQuery
	}
	return s.alerts.Acknowledge(ctx, id, userID)
}
