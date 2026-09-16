package model

import (
	"encoding/json"
	"time"
)

type APIResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
}
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	Roles        []string  `json:"roles,omitempty" db:"-"`
	Status       string    `json:"status" db:"status"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Role struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type ToolPermission struct {
	ID        int64     `json:"id" db:"id"`
	ToolID    int64     `json:"tool_id" db:"tool_id"`
	UserID    *int64    `json:"user_id,omitempty" db:"user_id"`
	RoleID    *int64    `json:"role_id,omitempty" db:"role_id"`
	Action    string    `json:"action" db:"action"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type AuditLog struct {
	ID           int64     `json:"id" db:"id"`
	RequestID    string    `json:"request_id" db:"request_id"`
	UserID       *int64    `json:"user_id,omitempty" db:"user_id"`
	ToolID       *int64    `json:"tool_id,omitempty" db:"tool_id"`
	DurationMS   int64     `json:"duration_ms" db:"duration_ms"`
	Status       string    `json:"status" db:"status"`
	DeniedReason string    `json:"denied_reason,omitempty" db:"denied_reason"`
	ParamsDigest string    `json:"params_digest,omitempty" db:"params_digest"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type CreateAuditLogRequest struct {
	RequestID    string `json:"request_id" binding:"required"`
	UserID       *int64 `json:"user_id"`
	ToolID       *int64 `json:"tool_id"`
	DurationMS   int64  `json:"duration_ms"`
	Status       string `json:"status" binding:"required"`
	DeniedReason string `json:"denied_reason"`
	Parameters   any    `json:"parameters"`
}
type MCPServer struct {
	ID                int64      `json:"id" db:"id"`
	Name              string     `json:"name" db:"name"`
	Description       string     `json:"description" db:"description"`
	Endpoint          string     `json:"endpoint" db:"endpoint"`
	Version           string     `json:"version" db:"version"`
	OwnerID           int64      `json:"owner_id" db:"owner_id"`
	Status            string     `json:"status" db:"status"`
	HealthStatus      string     `json:"health_status" db:"health_status"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at,omitempty" db:"last_health_check_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}
type RegisterServerRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description" binding:"max=1000"`
	Endpoint    string `json:"endpoint" binding:"required,url"`
	Version     string `json:"version" binding:"required,max=50"`
	OwnerID     int64  `json:"owner_id"`
}
type MCPTool struct {
	ID           int64           `json:"id" db:"id"`
	ServerID     int64           `json:"server_id" db:"server_id"`
	Name         string          `json:"name" db:"name"`
	Description  string          `json:"description" db:"description"`
	Category     string          `json:"category" db:"category"`
	Tags         []string        `json:"tags" db:"tags"`
	InputSchema  json.RawMessage `json:"input_schema" db:"input_schema"`
	Version      string          `json:"version" db:"version"`
	Published    bool            `json:"published" db:"published"`
	HealthStatus string          `json:"health_status" db:"health_status"`
	CallCount    int64           `json:"call_count" db:"call_count"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

type RegisterToolRequest struct {
	ServerID    int64           `json:"server_id" binding:"required"`
	Name        string          `json:"name" binding:"required,max=100"`
	Description string          `json:"description" binding:"max=1000"`
	Category    string          `json:"category" binding:"required,max=100"`
	Tags        []string        `json:"tags"`
	InputSchema json.RawMessage `json:"input_schema" binding:"required"`
	Version     string          `json:"version" binding:"required,max=50"`
}

type GrantToolPermissionRequest struct {
	UserID *int64  `json:"user_id"`
	RoleID *int64  `json:"role_id"`
	Action *string `json:"action"`
}

type ToolVersion struct {
	ID          int64           `json:"id" db:"id"`
	ToolID      int64           `json:"tool_id" db:"tool_id"`
	Version     string          `json:"version" db:"version"`
	InputSchema json.RawMessage `json:"input_schema" db:"input_schema"`
	Changelog   string          `json:"changelog" db:"changelog"`
	Status      string          `json:"status" db:"status"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

//网关代理

// McpToolView 返回给前端的工具简略视图
type McpToolView struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// McpListToolsResponse /gateway/tools/list 的返回体
type McpListToolsResponse struct {
	Tools []McpToolView `json:"tools"`
}

// ---- B9 MCP 接入配置生成 ----

// McpConfigResponse 为外部 Agent/IDE 生成的一站式接入配置：
// 网关地址 + 认证方式 + 操作说明 + 当前账号可调用的工具清单。
type McpConfigResponse struct {
	Protocol   string            `json:"protocol"` // 当前网关协议标识（custom-rest）
	Endpoint   string            `json:"endpoint"` // 网关 MCP 根地址，如 http://host:8080/mcp
	Auth       McpAuthGuide      `json:"auth"`
	Operations []McpOperationDoc `json:"operations"`
	Tools      []McpToolView     `json:"tools"`
}

// McpAuthGuide 认证接入指引。
type McpAuthGuide struct {
	Type       string `json:"type"`        // bearer
	TokenTTL   string `json:"token_ttl"`   // 令牌有效期，如 24h
	LoginPath  string `json:"login_path"`  // POST /api/auth/login
	HeaderName string `json:"header_name"` // Authorization
	Example    string `json:"example"`     // 登录获取令牌示例
}

// McpOperationDoc 单个网关操作的调用说明与示例。
type McpOperationDoc struct {
	Name            string `json:"name"`
	Method          string `json:"method"`
	Path            string `json:"path"`
	Description     string `json:"description"`
	RequestExample  string `json:"request_example"`
	ResponseExample string `json:"response_example"`
}

// McpToolCallRequest /gateway/tools/call 请求体
type McpToolCallRequest struct {
	Method    string                 `json:"method"`
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
}

// McpToolCallResponse /gateway/tools/call 上游MCP服务返回透传给调用方
type McpToolCallResponse struct {
	Content []map[string]interface{} `json:"content"`
	IsError bool                     `json:"is_error,omitempty"`
}
