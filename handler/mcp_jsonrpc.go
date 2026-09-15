package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"MCP-Nexus/middleware"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// 标准 MCP JSON-RPC 适配层（Streamable HTTP，无状态模式）。
// 协议参考：https://modelcontextprotocol.io/specification/2025-03-26/basic/transactions
// 复用现有 ProxyService 的鉴权 / 健康检查 / 转发 / 审计能力，只做协议翻译。

// mcpProtocolVersion 网关支持的 MCP 协议版本（向下列出可接受的客户端版本）。
var mcpSupportedVersions = map[string]bool{
	"2025-03-26": true,
	"2024-11-05": true,
}

const mcpDefaultVersion = "2024-11-05"

// JSON-RPC 2.0 标准错误码
const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
	rpcInternalError  = -32603
	// MCP 预留的服务端自定义错误区间 -32000 ~ -32099
	rpcToolDenied      = -32001 // 无权限
	rpcToolUnavailable = -32002 // 工具/服务离线或不可达
	rpcUpstreamError   = -32003 // 上游调用失败
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type mcpCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// MCPRPCHandler 标准 MCP JSON-RPC 端点处理器。
type MCPRPCHandler struct{ svc *service.ProxyService }

func NewMCPRPCHandler(svc *service.ProxyService) *MCPRPCHandler {
	return &MCPRPCHandler{svc: svc}
}

// Handle POST /mcp —— Streamable HTTP 单一端点（无状态，直接返回 application/json）。
func (h *MCPRPCHandler) Handle(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeRPCError(c, http.StatusBadRequest, nil, rpcParseError, "请求体读取失败")
		return
	}

	// 兼容批量调用（数组）；MVP 主链路为单条消息
	trimmed := trimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var batch []rpcRequest
		if err := json.Unmarshal(raw, &batch); err != nil {
			writeRPCError(c, http.StatusBadRequest, nil, rpcParseError, "JSON 解析失败")
			return
		}
		responses := make([]rpcResponse, 0, len(batch))
		for _, req := range batch {
			// 批量中的通知消息不产生响应
			if len(req.ID) == 0 {
				continue
			}
			if resp, ok := h.dispatch(c, req); ok {
				responses = append(responses, resp)
			}
		}
		c.JSON(http.StatusOK, responses)
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeRPCError(c, http.StatusBadRequest, nil, rpcParseError, "JSON 解析失败")
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		writeRPCError(c, http.StatusBadRequest, rawID(req.ID), rpcInvalidRequest, "无效的 JSON-RPC 请求")
		return
	}

	// 通知类消息（无 id）返回 202 Accepted，不带响应体
	if len(req.ID) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	resp, _ := h.dispatch(c, req)
	c.JSON(http.StatusOK, resp)
}

// dispatch 分发单条 JSON-RPC 消息。第二个返回值 false 表示这是通知（无需响应）。
func (h *MCPRPCHandler) dispatch(c *gin.Context, req rpcRequest) (rpcResponse, bool) {
	role := c.GetString("role")
	callerID := middleware.GetUserID(c)

	switch req.Method {
	case "initialize":
		return rpcResult(req.ID, h.initialize(req.Params)), true

	case "ping":
		return rpcResult(req.ID, gin.H{}), true

	case "tools/list":
		list, err := h.svc.ListTools(c.Request.Context(), role)
		if err != nil {
			return rpcErrResult(req.ID, rpcInternalError, "工具列表查询失败: "+err.Error()), true
		}
		tools := make([]gin.H, 0, len(list.Tools))
		for _, t := range list.Tools {
			schema := t.InputSchema
			if len(schema) == 0 || string(schema) == "null" {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			tools = append(tools, gin.H{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": schema,
			})
		}
		return rpcResult(req.ID, gin.H{"tools": tools}), true

	case "tools/call":
		var p mcpCallParams
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			return rpcErrResult(req.ID, rpcInvalidParams, "参数无效：需要 name 字段"), true
		}
		callReq := &model.McpToolCallRequest{
			Method:    "tools/call",
			ToolName:  p.Name,
			Arguments: p.Arguments,
		}
		res, err := h.svc.CallTool(c.Request.Context(), role, callerID, callReq)
		if err != nil {
			code, msg := mapProxyError(err)
			return rpcErrResult(req.ID, code, msg), true
		}
		// 上游执行失败也按 MCP 规范放在 result.isError=true
		return rpcResult(req.ID, gin.H{
			"content": res.Content,
			"isError": res.IsError,
		}), true

	default:
		return rpcErrResult(req.ID, rpcMethodNotFound, "不支持的方法: "+req.Method), true
	}
}

// initialize 返回协议版本、能力声明和服务端信息。
func (h *MCPRPCHandler) initialize(params json.RawMessage) gin.H {
	version := mcpDefaultVersion
	if len(params) > 0 {
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(params, &p); err == nil && mcpSupportedVersions[p.ProtocolVersion] {
			version = p.ProtocolVersion
		}
	}
	return gin.H{
		"protocolVersion": version,
		"capabilities": gin.H{
			"tools": gin.H{"listChanged": false},
		},
		"serverInfo": gin.H{
			"name":    "mcp-nexus-gateway",
			"version": "1.0.0",
		},
	}
}

// mapProxyError 将网关业务错误映射为 JSON-RPC 错误码与消息。
func mapProxyError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		return rpcToolDenied, "权限拒绝: " + err.Error()
	case errors.Is(err, service.ErrToolNotFound):
		return rpcInvalidParams, "工具不存在: " + err.Error()
	case errors.Is(err, service.ErrToolOffline):
		return rpcToolUnavailable, "工具已下线: " + err.Error()
	case errors.Is(err, service.ErrServerNotFound), errors.Is(err, service.ErrServerUnavailable):
		return rpcToolUnavailable, "服务不可用: " + err.Error()
	case errors.Is(err, service.ErrUpstreamTimeout):
		return rpcToolUnavailable, "上游超时: " + err.Error()
	case errors.Is(err, service.ErrUpstreamError):
		return rpcUpstreamError, "上游错误: " + err.Error()
	case errors.Is(err, repository.ErrNotFound):
		return rpcInvalidParams, "资源不存在: " + err.Error()
	default:
		return rpcInternalError, err.Error()
	}
}

func rpcResult(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: rawID(id), Result: result}
}

func rpcErrResult(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: rawID(id), Error: &rpcError{Code: code, Message: message}}
}

func writeRPCError(c *gin.Context, httpStatus int, id any, code int, message string) {
	c.JSON(httpStatus, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

// rawID 将原始 JSON id 原样回传（数字/字符串/null 均合法）；缺失时回 null。
func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return nil
	}
	return v
}

func trimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && isSpace(b[start]) {
		start++
	}
	for end > start && isSpace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}
