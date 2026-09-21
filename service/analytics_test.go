package service

import (
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"context"
	"testing"
	"time"
)

type fakeAnalyticsRepo struct{}

func (fakeAnalyticsRepo) Summary(context.Context, time.Time, time.Time) (*model.AnalyticsSummary, error) {
	return &model.AnalyticsSummary{TotalCalls: 10, SuccessfulCalls: 9}, nil
}
func (fakeAnalyticsRepo) Trends(context.Context, time.Time, time.Time, string, *int64) ([]model.AnalyticsPoint, error) {
	return []model.AnalyticsPoint{}, nil
}
func (fakeAnalyticsRepo) TopTools(context.Context, time.Time, time.Time, int) ([]model.ToolAnalyticsRank, error) {
	return []model.ToolAnalyticsRank{}, nil
}
func (fakeAnalyticsRepo) RejectReasons(context.Context, time.Time, time.Time, int) ([]model.RejectReasonStat, error) {
	return []model.RejectReasonStat{}, nil
}

type fakeAlertRepo struct{}

func (fakeAlertRepo) Create(context.Context, *model.AnomalyAlert) error { return nil }
func (fakeAlertRepo) List(context.Context, repository.AlertFilter) ([]*model.AnomalyAlert, int64, error) {
	return []*model.AnomalyAlert{}, 0, nil
}
func (fakeAlertRepo) Acknowledge(context.Context, int64, int64) error { return nil }
func TestAnalyticsOverviewStableShape(t *testing.T) {
	s := NewAnalyticsService(fakeAnalyticsRepo{}, fakeAlertRepo{})
	s.now = func() time.Time { return time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC) }
	v, e := s.Overview(context.Background(), 7)
	if e != nil || v.Summary.TotalCalls != 10 || v.Period.Days != 7 {
		t.Fatalf("unexpected overview %#v %v", v, e)
	}
}
