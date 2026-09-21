package model

import (
	"encoding/json"
	"time"
)

// ---- B11 Skills 适配器对接 ----
//
// Skills 适配任务已经为每个工具生成完整的描述/Schema/Endpoint，
// 网关把这些工具直接接入到 mcp_tools 表（与原生 MCP 工具一致，复用发现/权限/审计链路），
// 同时把调用元数据（endpoint/method/path/params）单独持久化到 mcp_tool_skills_specs 表。
//
// 与 B10 OpenAPI 的差异：
//   - OpenAPI spec 只存 method/path_template/params，baseURL 取自 Server.Endpoint（一个 Server 一个上游 API）
//   - Skills spec 额外存 endpoint，因为适配任务的工具可能指向不同的上游 Skills 服务实例
//
// 调用时 ProxyService 检测到工具存在 Skills spec 后，将 MCP 调用翻译为 HTTP 请求
// （baseURL = spec.Endpoint，空则 fallback server.Endpoint），发送到上游 Skills 服务。

// SkillsSpec 单个 Skills 工具的调用翻译元数据（持久化在 mcp_tool_skills_specs 表）。
// Params 以 json.RawMessage 存储（与 OpenAPISpec 一致），由 service 层按需解析为 SkillsParams。
type SkillsSpec struct {
	ID           int64           `json:"id" db:"id"`
	ToolID       int64           `json:"tool_id" db:"tool_id"`
	Endpoint     string          `json:"endpoint" db:"endpoint"`           // Skills 工具独立调用地址，空则用 server.Endpoint
	Method       string          `json:"method" db:"method"`               // HTTP 方法，默认 POST
	PathTemplate string          `json:"path_template" db:"path_template"` // 调用 path template
	Params       json.RawMessage `json:"params" db:"params"`               // 参数分类（path/query/body）
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

// SkillsParams 参数分类映射，与 OpenAPIParams 字段一致；复用同一语义：
//   - Path 元素名对应 path template 中的 {占位符}
//   - Query 元素拼到 query string
//   - Body 非空串表示 arguments[Body] 作为请求体透传
type SkillsParams struct {
	Path  []string `json:"path,omitempty"`
	Query []string `json:"query,omitempty"`
	Body  string   `json:"body,omitempty"`
}

// SkillsToolDef Skills 适配任务输出的单个工具描述（导入请求体中的一条工具）。
// 由导入侧（上游适配任务）提供完整字段，网关只需落地不需重新解析规范文档。
type SkillsToolDef struct {
	Name         string          `json:"name" binding:"required"`         // 工具名（同 Server 下唯一）
	Description  string          `json:"description"`                     // 工具描述
	InputSchema  json.RawMessage `json:"input_schema" binding:"required"` // 入参 JSON Schema
	Version      string          `json:"version"`                         // 工具版本（空则用 server.Version）
	Endpoint     string          `json:"endpoint"`                        // 调用地址（空则用 server.Endpoint）
	Method       string          `json:"method"`                          // HTTP 方法（空默认 POST）
	PathTemplate string          `json:"path_template"`                   // 调用 path（空则用 /tools/{name}/call 原生路径）
	Params       SkillsParams    `json:"params"`                          // 参数分类
}

// ImportSkillsRequest POST /api/servers/:id/import-skills 请求体。
// Tools 是适配任务已经计算好的工具定义列表；Published 控制导入后工具是否立即发布
// （默认 false：管理员审核后再显式发布，避免误曝光未校验的上游 Skills 工具）。
type ImportSkillsRequest struct {
	Tools     []SkillsToolDef `json:"tools" binding:"required,min=1"`
	Published bool            `json:"published"`
}

// ImportedSkillsEntry 导入结果中单个工具的落地信息。
type ImportedSkillsEntry struct {
	ToolID int64  `json:"tool_id"`
	Name   string `json:"name"`
	Action string `json:"action"` // created / updated
}

// ImportSkillsResult 导入汇总。
type ImportSkillsResult struct {
	ServerID int64                 `json:"server_id"`
	Imported []ImportedSkillsEntry `json:"imported"`
	Total    int                   `json:"total"`
}
