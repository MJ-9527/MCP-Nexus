package model

import (
	"encoding/json"
	"time"
)

// ---- B10 OpenAPI 包装器对接 ----
//
// OpenAPI/Swagger 规范的每个 path+method 被包装为一个 MCP 工具：
// - 工具本体注册在 mcp_tools 表（与原生 MCP 工具一致，复用发现/权限/审计链路）
// - 翻译元数据（method/path/params 分类）单独存储在 mcp_tool_openapi_specs 表
// - 调用时 ProxyService 检测到工具存在 OpenAPI 元数据后，将 MCP 请求翻译为实际 HTTP 请求
//   （path 参数填充 path template，query 参数拼到 query string，body 参数作为请求体）
//   发送到上游 API（即 MCPServer.Endpoint + path template），响应体包装为 McpToolCallResponse。

// OpenAPISpec 单个 OpenAPI operation 的翻译元数据（持久化在 mcp_tool_openapi_specs 表）。
// Params 以 json.RawMessage 存储（与 InputSchema 一致），由 service 层按需解析为 OpenAPIParams。
type OpenAPISpec struct {
	ID           int64           `json:"id" db:"id"`
	ToolID       int64           `json:"tool_id" db:"tool_id"`
	Method       string          `json:"method" db:"method"`
	PathTemplate string          `json:"path_template" db:"path_template"`
	Params       json.RawMessage `json:"params" db:"params"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

// OpenAPIParams 参数分类映射，描述 arguments 如何翻译为 HTTP 请求各部分。
// Path 中的元素名对应 path template 中的 {占位符}；Query 元素拼到 query string；
// Body 为空串表示无请求体，非空串表示 arguments[Body] 作为请求体透传。
type OpenAPIParams struct {
	Path  []string `json:"path,omitempty"`
	Query []string `json:"query,omitempty"`
	Body  string   `json:"body,omitempty"`
}

// ImportOpenAPIRequest POST /api/servers/:id/import-openapi 请求体。
// Spec 是原始 OpenAPI/Swagger JSON 规范；Published 控制导入后工具是否立即发布
// （默认 false：管理员审核后再显式发布，避免误曝光未校验的上游接口）。
type ImportOpenAPIRequest struct {
	Spec      json.RawMessage `json:"spec" binding:"required"`
	Published bool            `json:"published"`
}

// ImportedToolEntry 导入结果中单个工具的落地信息。
type ImportedToolEntry struct {
	ToolID int64  `json:"tool_id"`
	Name   string `json:"name"`
	Action string `json:"action"` // created / updated
}

// ImportOpenAPIResult 导入汇总。
type ImportOpenAPIResult struct {
	ServerID int64               `json:"server_id"`
	Imported []ImportedToolEntry `json:"imported"`
	Total    int                 `json:"total"`
}
