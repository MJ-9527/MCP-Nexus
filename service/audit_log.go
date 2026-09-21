package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidAuditLog = errors.New("invalid audit log")
)

type AuditLogService struct {
	logs      repository.AuditLogRepository
	analytics *AsyncAuditAnalyticsSink
}

func NewAuditLogService(logs repository.AuditLogRepository) *AuditLogService {
	return &AuditLogService{logs: logs}
}

func (s *AuditLogService) SetAnalytics(analytics *AsyncAuditAnalyticsSink) { s.analytics = analytics }

func (s *AuditLogService) Record(ctx context.Context, req model.CreateAuditLogRequest) (*model.AuditLog, error) {
	if s == nil || s.logs == nil || strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.Status) == "" || req.DurationMS < 0 || req.HTTPStatus < 0 || req.CostEstimate < 0 {
		return nil, ErrInvalidAuditLog
	}
	status := strings.TrimSpace(req.Status)
	digest, summary := "", ""
	if req.Parameters != nil {
		// B8 脱敏：参数落审计前递归掩码敏感字段（password/token/secret 等），再计算摘要
		data, err := json.Marshal(SanitizeValue(req.Parameters))
		if err != nil {
			return nil, ErrInvalidAuditLog
		}
		hash := sha256.Sum256(data)
		digest = hex.EncodeToString(hash[:])
		summary = "sha256:" + digest
	}
	log := &model.AuditLog{RequestID: strings.TrimSpace(req.RequestID), UserID: req.UserID, ToolID: req.ToolID, ServerID: req.ServerID, ToolName: strings.TrimSpace(req.ToolName), CallerRole: strings.TrimSpace(req.CallerRole), DurationMS: req.DurationMS, Status: status, HTTPStatus: req.HTTPStatus, DeniedReason: strings.TrimSpace(req.DeniedReason), RejectReason: strings.TrimSpace(req.RejectReason), ParamsSummary: summary, ParamsSensitiveMasked: req.Parameters != nil, ParamsDigest: digest, CostEstimate: req.CostEstimate}
	if err := s.logs.Create(ctx, log); err != nil {
		return nil, err
	}
	if s.analytics != nil {
		s.analytics.Enqueue(log)
	}
	return log, nil
}

func (s *AuditLogService) List(ctx context.Context, filter repository.AuditLogFilter) ([]*model.AuditLog, int64, error) {
	if s == nil || s.logs == nil || filter.Page < 0 || filter.PageSize < 0 {
		return nil, 0, ErrInvalidAuditLog
	}
	return s.logs.List(ctx, filter)
}
