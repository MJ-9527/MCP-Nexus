package service

import (
	"fmt"
	"sync"
	"time"
)

type AlertRecord struct {
	AlertID   string    `json:"alert_id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	ToolID    int64     `json:"tool_id"`
	ToolName  string    `json:"tool_name"`
	Message   string    `json:"message"`
	Triggered time.Time `json:"triggered_at"`
	Handled   bool      `json:"handled"`
}

type AlertService struct {
	mu     sync.RWMutex
	alerts []*AlertRecord
	nextID int64
}

func NewAlertService() *AlertService {
	return &AlertService{alerts: make([]*AlertRecord, 0)}
}

func (s *AlertService) CheckAndCreate(toolID int64, toolName string, callCount int, threshold int) {
	if callCount <= threshold {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	alert := &AlertRecord{
		AlertID:   fmt.Sprintf("alert-%03d", s.nextID),
		Type:      "sensitive_tool_high_frequency",
		Severity:  "high",
		ToolID:    toolID,
		ToolName:  toolName,
		Message:   fmt.Sprintf("tool %s called %d times, threshold %d", toolName, callCount, threshold),
		Triggered: time.Now(),
		Handled:   false,
	}
	s.alerts = append(s.alerts, alert)
}

func (s *AlertService) List(handled *bool) []*AlertRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*AlertRecord, 0, len(s.alerts))
	for _, a := range s.alerts {
		if handled != nil && a.Handled == *handled {
			out = append(out, a)
		} else if handled == nil {
			out = append(out, a)
		}
	}
	return out
}

func (s *AlertService) Acknowledge(alertID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.alerts {
		if a.AlertID == alertID {
			a.Handled = true
			return true
		}
	}
	return false
}

