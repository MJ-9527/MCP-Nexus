package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// CallTimeout 单次工具调用的整体超时（网关侧保护，防止上游故障长期占用资源）。
const CallTimeout = 10 * time.Second

type ProxyService struct {
	serverRepo    repository.ServerRepository
	toolRepo      repository.ToolRepository
	permissionCli PermissionClient
	mcpClient     *client.MCPClient
	audit         *AuditLogService // B8 审计埋点：可 nil（无审计依赖时调用链照常工作）
}

func NewProxyService(serverRepo repository.ServerRepository,
	toolRepo repository.ToolRepository,
	permCli PermissionClient) *ProxyService {
	return &ProxyService{
		serverRepo:    serverRepo,
		toolRepo:      toolRepo,
		permissionCli: permCli,
		mcpClient:     client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes),
	}
}

// SetAudit 注入审计服务（B8）。必须在首次调用前设置。
func (s *ProxyService) SetAudit(audit *AuditLogService) { s.audit = audit }

// ListTools 工具发现：仅返回【Server 在线 + 工具已发布 + 有 view/call 权限】的工具（B1/B3/B6）。
// 权限取 view 与 call 两个 action 的并集：可调用者必然可见，单独授予 view 的账号也可见。
func (s *ProxyService) ListTools(ctx context.Context, role string, userID int64) (*model.McpListToolsResponse, error) {
	allowed := make(map[string]bool)
	for _, action := range []string{PermissionActionView, PermissionActionCall} {
		allowedToolNames, err := s.permissionCli.GetAllowedToolNames(ctx, role, userID, action)
		if err != nil {
			return nil, fmt.Errorf("query permission error: %w", err)
		}
		for _, name := range allowedToolNames {
			allowed[name] = true
		}
	}

	// 仅已发布工具（下线工具不出现在发现列表）
	published := true
	tools, err := s.toolRepo.List(ctx, repository.ToolFilter{Published: &published})
	if err != nil {
		return nil, fmt.Errorf("list tools error: %w", err)
	}

	servers, err := s.serverRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers error: %w", err)
	}
	onlineServerIDs := make(map[int64]bool, len(servers))
	for _, srv := range servers {
		if srv.HealthStatus == "online" {
			onlineServerIDs[srv.ID] = true
		}
	}

	viewList := make([]model.McpToolView, 0)
	for _, t := range tools {
		if onlineServerIDs[t.ServerID] && allowed[t.Name] {
			viewList = append(viewList, model.McpToolView{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
	return &model.McpListToolsResponse{Tools: viewList}, nil
}

// 审计状态分类（B8）：成功放行 / 权限拒绝 / 其他失败。
const (
	AuditStatusSuccess = "success"
	AuditStatusDenied  = "denied"
	AuditStatusFailed  = "failed"
)

// CallTool 调用前检查（B3/B6）→ 转发下游（B2）→ 统一业务错误（B4）→ 审计埋点（B8）。
func (s *ProxyService) CallTool(ctx context.Context, role string, userID int64, req *model.McpToolCallRequest, requestID string) (*model.McpToolCallResponse, error) {
	startedAt := time.Now()
	var toolID int64
	resp, err := s.callTool(ctx, role, userID, req, requestID, &toolID)
	s.recordCallAudit(requestID, userID, toolID, startedAt, err, req)
	return resp, err
}

// recordCallAudit 异步落审计：不阻塞调用链；审计失败仅记日志不影响调用结果（B8）。
func (s *ProxyService) recordCallAudit(requestID string, userID int64, toolID int64, startedAt time.Time, err error, req *model.McpToolCallRequest) {
	if s.audit == nil || requestID == "" {
		return
	}
	status, reason := AuditStatusSuccess, ""
	if err != nil {
		if errors.Is(err, ErrPermissionDenied) {
			status, reason = AuditStatusDenied, err.Error()
		} else {
			status, reason = AuditStatusFailed, err.Error()
		}
	}
	id := userID
	var toolPtr *int64
	if toolID > 0 {
		toolPtr = &toolID
	}
	entry := model.CreateAuditLogRequest{
		RequestID:    requestID,
		UserID:       &id,
		ToolID:       toolPtr,
		DurationMS:   time.Since(startedAt).Milliseconds(),
		Status:       status,
		DeniedReason: reason,
		Parameters:   req.Arguments, // Record 内部统一脱敏后摘要，原文不落库
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, auditErr := s.audit.Record(ctx, entry); auditErr != nil {
			log.Printf("[audit] 落审计失败 request_id=%s status=%s err=%v", requestID, status, auditErr)
		}
	}()
}

// callTool 核心调用链，toolID 用于审计记录。
func (s *ProxyService) callTool(ctx context.Context, role string, userID int64, req *model.McpToolCallRequest, requestID string, toolID *int64) (*model.McpToolCallResponse, error) {
	// 1. RBAC：角色授权或用户直授是否有权调用该工具（action=call）
	allowedToolNames, err := s.permissionCli.GetAllowedToolNames(ctx, role, userID, PermissionActionCall)
	if err != nil {
		return nil, fmt.Errorf("query permission error: %w", err)
	}
	hasPermission := false
	for _, name := range allowedToolNames {
		if name == req.ToolName {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		// 保留拒绝原因（B6）：账号身份与所需操作
		return nil, fmt.Errorf("%w: user %d (role %q) is not authorized to call tool %q", ErrPermissionDenied, userID, role, req.ToolName)
	}

	// 2. 工具存在且已发布
	tool, err := s.toolRepo.FindPublishedByName(ctx, req.ToolName)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrToolNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find tool error: %w", err)
	}
	*toolID = tool.ID

	// 3. 工具未被明确下线（unknown=未检测可放行；只有明确 offline 才拒绝）
	if tool.HealthStatus == "offline" {
		return nil, fmt.Errorf("%w: tool %s health is %s", ErrToolOffline, tool.Name, tool.HealthStatus)
	}

	// 4. 所属 Server 存在、已激活且健康在线
	server, err := s.serverRepo.FindByID(ctx, tool.ServerID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServerNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find server error: %w", err)
	}
	if server.Status != "active" {
		return nil, fmt.Errorf("%w: server %s status is %s", ErrServerUnavailable, server.Name, server.Status)
	}
	if server.HealthStatus != "online" {
		return nil, fmt.Errorf("%w: server %s health is %s", ErrServerUnavailable, server.Name, server.HealthStatus)
	}

	// 5. 整体超时保护 + 转发（响应透传）
	callCtx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	respBody, err := s.mcpClient.PostJSON(callCtx, server.Endpoint, req, map[string]string{
		"request_id": requestID,
	})
	if err != nil {
		return nil, mapUpstreamError(err)
	}

	var callResp model.McpToolCallResponse
	if err := json.Unmarshal(respBody, &callResp); err != nil {
		return nil, fmt.Errorf("%w: parse upstream response: %v", ErrUpstreamError, err)
	}
	return &callResp, nil
}

// mapUpstreamError 将传输层错误映射为网关业务错误（B4）。
func mapUpstreamError(err error) error {
	switch {
	case errors.Is(err, client.ErrUpstreamTimeout):
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, errors.Unwrap(err))
	case errors.Is(err, client.ErrServerUnreachable):
		return fmt.Errorf("%w: %v", ErrServerUnavailable, errors.Unwrap(err))
	case errors.Is(err, client.ErrBodyTooLarge):
		return fmt.Errorf("%w: %v", ErrUpstreamError, errors.Unwrap(err))
	default:
		return fmt.Errorf("%w: %v", ErrUpstreamError, err)
	}
}
