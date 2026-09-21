package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidAuditLog = errors.New("invalid audit log")
)

type AuditLogService struct {
	logs   repository.AuditLogRepository
	writer *BatchAuditWriter // B14 异步批量写入器（可选；非 nil 时 Submit 走异步入队）
	analytics *AsyncAuditAnalyticsSink
}

func NewAuditLogService(logs repository.AuditLogRepository) *AuditLogService {
	return &AuditLogService{logs: logs}
}

// SetBatchWriter 注入异步批量写入器（B14）。必须在首次 Submit 前调用，
// 调用方负责 Start/Stop 生命周期。设为 nil 表示禁用批量，回退同步路径。
func (s *AuditLogService) SetBatchWriter(w *BatchAuditWriter) {
	s.writer = w
}

// buildAuditLog 共享：校验入参 + 脱敏 + 摘要 + 构造 AuditLog 对象。
// Record（同步）与 Submit（异步）共用，保证两条路径的脱敏与摘要一致。
func buildAuditLog(req model.CreateAuditLogRequest) (*model.AuditLog, error) {
	if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.Status) == "" || req.DurationMS < 0 || req.HTTPStatus < 0 || req.CostEstimate < 0 {
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
	return &model.AuditLog{
		RequestID:             strings.TrimSpace(req.RequestID),
		UserID:                req.UserID,
		ToolID:                req.ToolID,
		ServerID:              req.ServerID,
		ToolName:              strings.TrimSpace(req.ToolName),
		CallerRole:            strings.TrimSpace(req.CallerRole),
		DurationMS:            req.DurationMS,
		Status:                status,
		HTTPStatus:            req.HTTPStatus,
		DeniedReason:          strings.TrimSpace(req.DeniedReason),
		RejectReason:          strings.TrimSpace(req.RejectReason),
		ParamsSummary:         summary,
		ParamsSensitiveMasked: req.Parameters != nil,
		ParamsDigest:          digest,
		CostEstimate:          req.CostEstimate,
	}, nil
}

func (s *AuditLogService) Record(ctx context.Context, req model.CreateAuditLogRequest) (*model.AuditLog, error) {
	if s == nil || s.logs == nil {
		return nil, ErrInvalidAuditLog
	}
	log, err := buildAuditLog(req)
	if err != nil {
		return nil, err
	}
	if err := s.logs.Create(ctx, log); err != nil {
		return nil, err
	}
	if s.analytics != nil {
		s.analytics.Enqueue(log)
	}
	return log, nil
}

// Submit 异步入队（B14）：脱敏 + 摘要后立即投递到批量队列。
// 调用链不阻塞：队满时由 writer 丢弃并增加 dropped 计数。
// writer 未注入时回退为同步 Create，保持原有行为（便于无批量依赖的单测）。
func (s *AuditLogService) Submit(req model.CreateAuditLogRequest) {
	if s == nil || s.logs == nil {
		return
	}
	log, err := buildAuditLog(req)
	if err != nil {
		// 校验失败仅记日志，不影响调用链
		return
	}
	if s.writer != nil {
		s.writer.Submit(log)
		return
	}
	// 回退：未注入批量写入器时走同步路径（容灾降级）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = s.logs.Create(ctx, log)
}

func (s *AuditLogService) List(ctx context.Context, filter repository.AuditLogFilter) ([]*model.AuditLog, int64, error) {
	if s == nil || s.logs == nil || filter.Page < 0 || filter.PageSize < 0 || filter.PageSize > 100 {
		return nil, 0, ErrInvalidAuditLog
	}
	return s.logs.List(ctx, filter)
}
