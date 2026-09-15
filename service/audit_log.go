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

type AuditLogService struct{ logs repository.AuditLogRepository }

func NewAuditLogService(logs repository.AuditLogRepository) *AuditLogService {
	return &AuditLogService{logs: logs}
}

func (s *AuditLogService) Record(ctx context.Context, req model.CreateAuditLogRequest) (*model.AuditLog, error) {
	if s == nil || s.logs == nil || strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.Status) == "" || req.DurationMS < 0 {
		return nil, ErrInvalidAuditLog
	}
	status := strings.TrimSpace(req.Status)
	digest := ""
	if req.Parameters != nil {
		data, err := json.Marshal(req.Parameters)
		if err != nil {
			return nil, ErrInvalidAuditLog
		}
		hash := sha256.Sum256(data)
		digest = hex.EncodeToString(hash[:])
	}
	log := &model.AuditLog{RequestID: strings.TrimSpace(req.RequestID), UserID: req.UserID, ToolID: req.ToolID, DurationMS: req.DurationMS, Status: status, DeniedReason: strings.TrimSpace(req.DeniedReason), ParamsDigest: digest}
	if err := s.logs.Create(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *AuditLogService) List(ctx context.Context, filter repository.AuditLogFilter) ([]*model.AuditLog, int64, error) {
	if s == nil || s.logs == nil || filter.Limit < 0 || filter.Offset < 0 {
		return nil, 0, ErrInvalidAuditLog
	}
	return s.logs.List(ctx, filter)
}
