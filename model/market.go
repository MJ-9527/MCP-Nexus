package model

import (
	"encoding/json"
	"time"
)

// ============ 工具市场 ============

// MarketToolItem 市场列表条目
type MarketToolItem struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Tags         []string  `json:"tags"`
	Version      string    `json:"version"`
	HealthStatus string    `json:"health_status"`
	ServerName   string    `json:"server_name"`
	CallCount    int64     `json:"call_count"`
	AvgRating    float64   `json:"avg_rating"`
	ReviewCount  int64     `json:"review_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// MarketListResponse 市场列表返回体（带分页）
type MarketListResponse struct {
	Items      []MarketToolItem `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Total      int64            `json:"total"`
	TotalPages int              `json:"total_pages"`
}

// MarketToolDetail 工具详情：版本、健康、Schema、调用量、评分统计、上游 Server 信息
type MarketToolDetail struct {
	MCPTool
	ServerName      string     `json:"server_name"`
	ServerEndpoint  string     `json:"server_endpoint"`
	ServerHealth    string     `json:"server_health"`
	HealthCheckedAt *time.Time `json:"health_checked_at,omitempty"`
	CallCount30d    int64      `json:"call_count_30d"`
	SuccessRate30d  float64    `json:"success_rate_30d"`
	AvgRating       float64    `json:"avg_rating"`
	ReviewCount     int64      `json:"review_count"`
}

// Review 评分评论
type Review struct {
	ID        int64     `json:"id" db:"id"`
	ToolID    int64     `json:"tool_id" db:"tool_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Username  string    `json:"username" db:"username"`
	Rating    int       `json:"rating" db:"rating"`
	Comment   string    `json:"comment" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UpsertReviewRequest 提交/更新评分评论
type UpsertReviewRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"max=2000"`
}

// RankingItem 调用量排行条目
type RankingItem struct {
	ToolName  string `json:"tool_name"`
	CallCount int64  `json:"call_count"`
}

// ConfigSnippetResponse 一键生成的 MCP 接入配置片段
type ConfigSnippetResponse struct {
	ToolName string          `json:"tool_name"`
	Snippet  json.RawMessage `json:"snippet"`
	Notes    []string        `json:"notes"`
}

// ============ OpenAPI 自动包装器 ============

// OpenAPIImportRequest OpenAPI 导入请求
type OpenAPIImportRequest struct {
	// Spec：OpenAPI 3.x JSON 或 YAML 原文
	Spec string `json:"spec" binding:"required"`
	// BaseURL 覆盖 spec 里的 server 地址（可选）
	BaseURL string `json:"base_url" binding:"omitempty,url"`
	// AutoRegister：true 时把生成的工具注册进网关（创建 mcp_server + mcp_tools）
	AutoRegister bool `json:"auto_register"`
	// RegisterName：auto_register 时使用的 Server 名称（默认取 spec.info.title）
	RegisterName string `json:"register_name"`
}

// GeneratedFile 生成的骨架文件
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// OpenAPIImportResult 导入结果
type OpenAPIImportResult struct {
	InfoTitle    string          `json:"info_title"`
	BaseURL      string          `json:"base_url"`
	AuthType     string          `json:"auth_type"` // none / api_key / bearer
	Tools        []MCPTool       `json:"tools"`     // 生成的 MCP 工具定义（已落库则为真实 ID）
	Files        []GeneratedFile `json:"files"`     // Go MCP Server 骨架
	ServerID     *int64          `json:"server_id,omitempty"`
	EnvTemplate  string          `json:"env_template"`
	StartupGuide string          `json:"startup_guide"`
}

// ============ Skills 适配器 ============

// SkillDefinition 规范化 Skill 描述
type SkillDefinition struct {
	Name        string                 `json:"name" binding:"required,max=100"`
	Description string                 `json:"description" binding:"max=1000"`
	Version     string                 `json:"version" binding:"required,max=50"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
	// Endpoint：Skill 执行入口 URL，网关调用时转发
	Endpoint string   `json:"endpoint" binding:"required,url"`
	Category string   `json:"category" binding:"max=100"`
	Tags     []string `json:"tags"`
}

// SkillImportRequest 批量导入 Skills
type SkillImportRequest struct {
	Skills []SkillDefinition `json:"skills" binding:"required,min=1"`
	// RegisterName：虚拟 Server 名称（默认 "skills-imported"），同名复用实现幂等
	RegisterName string `json:"register_name"`
}

// SkillImportResult 导入结果
type SkillImportResult struct {
	ServerID int64           `json:"server_id"`
	Tools    []MCPTool       `json:"tools"`
	Snippet  json.RawMessage `json:"snippet"`
}
