<<<<<<< HEAD
﻿package service

import (
	"context"
	"fmt"
	"time"
)

// AlertPayload represents an alert for analytics reporting.
type AlertPayload struct {
	AlertID   string `json:"alert_id"`
	Type      string `json:"type"`
	Severity  string `json:"severity"`
	ToolName  string `json:"tool_name"`
	Message   string `json:"message"`
	Triggered string `json:"triggered_at"`
}

// AuditEntry is a lightweight audit record used by analytics.
type AuditEntry struct {
	RequestID  string    `json:"request_id"`
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	UserID     int64     `json:"user_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	Role       string    `json:"role,omitempty"`
	StatusCode int       `json:"status_code"`
	Code       string    `json:"code"`
	Message    string    `json:"message,omitempty"`
	LatencyMs  int64     `json:"latency_ms"`
	ToolID     int64     `json:"tool_id,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
}

// AnalyticsService provides usage statistics from audit entries.
type AnalyticsService struct {
	entries []AuditEntry
}

func NewAnalyticsService() *AnalyticsService {
	return &AnalyticsService{}
}

// AddEntries appends audit entries for in-memory analytics.
func (s *AnalyticsService) AddEntries(entries []AuditEntry) {
	s.entries = append(s.entries, entries...)
}

// Overview returns summary statistics from audit entries.
func (s *AnalyticsService) Overview(ctx context.Context, days int) (map[string]any, error) {
	summary := computeSummary(s.entries, days)
	topTools := topToolsByCalls(s.entries)
	dailyTrend := dailyTrend(s.entries, days)
	return map[string]any{
		"period": map[string]any{"days": days},
		"summary": map[string]any{
			"total_calls":     summary.totalCalls,
			"successful_calls": summary.successCalls,
			"failed_calls":    summary.failCalls,
			"rejected_calls":  summary.rejectedCalls,
			"success_rate":    summary.successRate,
			"avg_latency_ms":  summary.avgLatency,
		},
		"top_tools":   topTools,
		"daily_trend": dailyTrend,
		"alerts":      nil,
	}, nil
}

type summaryStats struct {
	totalCalls   int64
	successCalls int64
	failCalls    int64
	rejectedCalls int64
	totalLatency int64
	successRate  float64
	avgLatency   int64
}

func computeSummary(entries []AuditEntry, days int) summaryStats {
	var s summaryStats
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, e := range entries {
		if e.Timestamp.Before(cutoff) {
			continue
		}
		s.totalCalls++
		s.totalLatency += e.LatencyMs
		switch e.Code {
		case "OK":
			s.successCalls++
		case "RATE_LIMITED", "FORBIDDEN", "UNAUTHORIZED":
			s.rejectedCalls++
		default:
			s.failCalls++
		}
	}
	if s.totalCalls > 0 {
		s.successRate = float64(s.successCalls) / float64(s.totalCalls) * 100
		s.avgLatency = s.totalLatency / s.totalCalls
	}
	return s
}

func topToolsByCalls(entries []AuditEntry) []map[string]any {
	type toolStat struct {
		toolID  int64
		name    string
		calls   int64
		success int64
	}
	stats := make(map[int64]*toolStat)
	for _, e := range entries {
		ts, ok := stats[e.ToolID]
		if !ok {
			ts = &toolStat{toolID: e.ToolID, name: e.ToolName}
			stats[e.ToolID] = ts
		}
		ts.calls++
		if e.Code == "OK" {
			ts.success++
		}
	}
	list := make([]*toolStat, 0, len(stats))
	for _, ts := range stats {
		list = append(list, ts)
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].calls > list[i].calls {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	out := make([]map[string]any, 0, len(list))
	for _, ts := range list {
		rate := 0.0
		if ts.calls > 0 {
			rate = float64(ts.success) / float64(ts.calls) * 100
		}
		out = append(out, map[string]any{
			"tool_id":      ts.toolID,
			"tool_name":    ts.name,
			"call_count":   ts.calls,
			"success_rate": rate,
		})
	}
	return out
}

func dailyTrend(entries []AuditEntry, days int) []map[string]any {
	type dayStat struct {
		date     string
		calls    int64
		success  int64
		rejected int64
	}
	stats := make(map[string]*dayStat)
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, e := range entries {
		if e.Timestamp.Before(cutoff) {
			continue
		}
		day := e.Timestamp.Format("2006-01-02")
		ds, ok := stats[day]
		if !ok {
			ds = &dayStat{date: day}
			stats[day] = ds
		}
		ds.calls++
		if e.Code == "OK" {
			ds.success++
		} else if e.Code == "RATE_LIMITED" || e.Code == "FORBIDDEN" || e.Code == "UNAUTHORIZED" {
			ds.rejected++
		}
	}
	out := make([]map[string]any, 0, len(stats))
	for _, ds := range stats {
		out = append(out, map[string]any{
			"date":     ds.date,
			"calls":    ds.calls,
			"success":  ds.success,
			"rejected": ds.rejected,
		})
	}
	return out
}

// TriggerAlert creates an alert payload when call count exceeds threshold.
func TriggerAlert(toolID int64, toolName string, callCount int, threshold int) *AlertPayload {
	if callCount <= threshold {
		return nil
	}
	return &AlertPayload{
		AlertID:   fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Type:      "sensitive_tool_high_frequency",
		Severity:  "high",
		ToolName:  toolName,
		Message:   fmt.Sprintf("工具 %s 在1小时内被调用 %d 次，超过阈值 %d", toolName, callCount, threshold),
		Triggered: time.Now().UTC().Format(time.RFC3339),
	}
}
=======
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
>>>>>>> origin/pull-request
