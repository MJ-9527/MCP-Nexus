package repository

import (
	"MCP-Nexus/model"
	"context"
	"time"
)

type AnalyticsRepository interface {
	Summary(context.Context, time.Time, time.Time) (*model.AnalyticsSummary, error)
	Trends(context.Context, time.Time, time.Time, string, *int64) ([]model.AnalyticsPoint, error)
	TopTools(context.Context, time.Time, time.Time, int) ([]model.ToolAnalyticsRank, error)
	RejectReasons(context.Context, time.Time, time.Time, int) ([]model.RejectReasonStat, error)
}
type AlertFilter struct {
	Status   string
	Severity string
	Page     int
	PageSize int
}
type AlertRepository interface {
	Create(context.Context, *model.AnomalyAlert) error
	List(context.Context, AlertFilter) ([]*model.AnomalyAlert, int64, error)
	Acknowledge(context.Context, int64, int64) error
}
