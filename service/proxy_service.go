package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/permission"
	"MCP-Nexus/repository"
)

var (
	ErrPermissionUnavailable = errors.New("permission service unavailable")
	ErrPermissionDenied      = errors.New("permission denied")
	ErrToolOffline           = errors.New("tool offline")
	ErrServerUnavailable     = errors.New("server unavailable")
	ErrUpstreamTimeout       = errors.New("upstream timeout")
	ErrUpstreamError         = errors.New("upstream error")
)

const proxyCallTimeout = 5 * time.Second

// ProxyService 网关代理服务：工具发现 + 工具调用转发。
type ProxyService struct {
	servers    repository.ServerRepository
	tools      repository.ToolRepository
	perm       permission.PermissionClient
	audit      *AuditService
	httpClient *http.Client
	now        func() time.Time
}

func NewProxyService(servers repository.ServerRepository, tools repository.ToolRepository, perm permission.PermissionClient, audit *AuditService) *ProxyService {
	return &ProxyService{
		servers:    servers,
		tools:      tools,
		perm:       perm,
		audit:      audit,
		httpClient: &http.Client{Timeout: proxyCallTimeout},
		now:        time.Now,
	}
}

// recordAudit 记录一次工具调用的审计日志（异步）。
func (s *ProxyService) recordAudit(ctx context.Context, role string, req *model.McpToolCallRequest, callerID int64, latencyMS int32, success bool, httpStatus int32, rejectReason string) {
	if s.audit == nil {
		return
	}
	serverID := int64(0)
	entry := &model.AuditLog{
		RequestID:    requestIDFromCtx(ctx),
		CallerID:     callerID,
		Role:         role,
		ToolName:     req.ToolName,
		ServerID:     serverID,
		ArgsSummary:  SummarizeArgs(req.Arguments),
		CalledAt:     s.now(),
		LatencyMS:    latencyMS,
		Success:      success,
		HTTPStatus:   httpStatus,
		RejectReason: rejectReason,
	}
	s.audit.RecordAsync(entry)
}

// requestIDFromCtx 从 context 中取 request_id（由 request_id 中间件注入）。
func requestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value("request_id").(string); ok {
		return v
	}
	return ""
}

// ListTools 返回当前角色有权调用、且 published + server active/online 的工具。
func (s *ProxyService) ListTools(ctx context.Context, role string) (*model.McpListToolsResponse, error) {
	if s == nil || s.tools == nil || s.servers == nil || s.perm == nil {
		return nil, ErrInvalidTool
	}
	allowed, err := s.perm.GetAllowedToolNamesByRole(ctx, role)
	if err != nil {
		return nil, ErrPermissionUnavailable
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, n := range allowed {
		allowedSet[n] = true
	}

	published := true
	tools, err := s.tools.List(ctx, repository.ToolFilter{Published: &published})
	if err != nil {
		return nil, err
	}

	resp := &model.McpListToolsResponse{Tools: []model.McpToolView{}}
	for _, t := range tools {
		// 手动下线的工具不可见；unknown/online 由 server 健康兜底
		if t.HealthStatus == "offline" {
			continue
		}
		// 通配 "*" 表示拥有全部工具权限
		if !allowedSet["*"] && !allowedSet[t.Name] {
			continue
		}
		server, err := s.servers.FindByID(ctx, t.ServerID)
		if err != nil || server.Status != "active" || server.HealthStatus != "online" {
			continue
		}
		resp.Tools = append(resp.Tools, toMcpToolView(t))
	}
	return resp, nil
}

// CallTool 校验权限后转发到下游 MCP Server，透传上游 MCP 协议响应。
// callerID 由 handler 从 JWT 注入，用于审计日志。
func (s *ProxyService) CallTool(ctx context.Context, role string, callerID int64, req *model.McpToolCallRequest) (resp *model.McpToolCallResponse, err error) {
	if s == nil || s.tools == nil || s.servers == nil || s.perm == nil || req == nil || strings.TrimSpace(req.ToolName) == "" {
		return nil, ErrInvalidTool
	}

	start := s.now()
	var rejectReason string
	var httpStatus int32
	defer func() {
		latencyMS := int32(time.Since(start).Milliseconds())
		success := err == nil
		if !success && rejectReason == "" {
			rejectReason = err.Error()
		}
		s.recordAudit(ctx, role, req, callerID, latencyMS, success, httpStatus, rejectReason)
	}()

	allowed, err := s.perm.GetAllowedToolNamesByRole(ctx, role)
	if err != nil {
		rejectReason = ErrPermissionUnavailable.Error()
		return nil, ErrPermissionUnavailable
	}
	if !containsString(allowed, "*") && !containsString(allowed, req.ToolName) {
		rejectReason = ErrPermissionDenied.Error()
		return nil, ErrPermissionDenied
	}

	tool, err := s.tools.FindPublishedByName(ctx, req.ToolName)
	if errors.Is(err, repository.ErrNotFound) {
		rejectReason = ErrToolNotFound.Error()
		return nil, ErrToolNotFound
	}
	if err != nil {
		return nil, err
	}
	// 仅拒绝手动下线；unknown（刚注册未探测）放行，由 server 健康兜底
	if tool.HealthStatus == "offline" {
		rejectReason = ErrToolOffline.Error()
		return nil, ErrToolOffline
	}

	server, err := s.servers.FindByID(ctx, tool.ServerID)
	if errors.Is(err, repository.ErrNotFound) {
		rejectReason = ErrServerNotFound.Error()
		return nil, ErrServerNotFound
	}
	if err != nil {
		return nil, err
	}
	if server.Status != "active" || server.HealthStatus != "online" {
		rejectReason = ErrServerUnavailable.Error()
		return nil, ErrServerUnavailable
	}

	body, _ := json.Marshal(map[string]any{"arguments": req.Arguments})
	url := fmt.Sprintf("%s/tools/%s/call", strings.TrimRight(server.Endpoint, "/"), tool.Name)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			rejectReason = ErrUpstreamTimeout.Error()
			return nil, ErrUpstreamTimeout
		}
		rejectReason = ErrUpstreamError.Error()
		return nil, ErrUpstreamError
	}
	defer httpResp.Body.Close()
	httpStatus = int32(httpResp.StatusCode)

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		rejectReason = ErrUpstreamError.Error()
		return nil, ErrUpstreamError
	}
	if httpResp.StatusCode >= 400 {
		rejectReason = fmt.Sprintf("%s: status %d", ErrUpstreamError, httpResp.StatusCode)
		return nil, fmt.Errorf("%w: status %d", ErrUpstreamError, httpResp.StatusCode)
	}

	// 透传上游 MCP 协议响应（{content:[...], is_error:bool}）
	var parsed model.McpToolCallResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		rejectReason = ErrUpstreamError.Error()
		return nil, ErrUpstreamError
	}
	return &parsed, nil
}

func toMcpToolView(t *model.MCPTool) model.McpToolView {
	return model.McpToolView{
		Name: t.Name, Description: t.Description, InputSchema: t.InputSchema,
	}
}

func containsString(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}
