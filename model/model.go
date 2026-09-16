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
	ID                    int64     `json:"id" db:"id"`
	RequestID             string    `json:"request_id" db:"request_id"`
	UserID                *int64    `json:"user_id,omitempty" db:"user_id"`
	ToolID                *int64    `json:"tool_id,omitempty" db:"tool_id"`
	ServerID              *int64    `json:"server_id,omitempty" db:"server_id"`
	ToolName              string    `json:"tool_name,omitempty" db:"tool_name"`
	CallerRole            string    `json:"caller_role,omitempty" db:"caller_role"`
	DurationMS            int64     `json:"duration_ms" db:"duration_ms"`
	Status                string    `json:"status" db:"status"`
	HTTPStatus            int       `json:"http_status" db:"http_status"`
	DeniedReason          string    `json:"denied_reason,omitempty" db:"denied_reason"`
	RejectReason          string    `json:"reject_reason,omitempty" db:"reject_reason"`
	ParamsSummary         string    `json:"params_summary,omitempty" db:"params_summary"`
	ParamsSensitiveMasked bool      `json:"params_sensitive_masked" db:"params_sensitive_masked"`
	ParamsDigest          string    `json:"params_digest,omitempty" db:"params_digest"`
	CostEstimate          float64   `json:"cost_estimate" db:"cost_estimate"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
}

type CreateAuditLogRequest struct {
	RequestID    string  `json:"request_id" binding:"required"`
	UserID       *int64  `json:"user_id"`
	ToolID       *int64  `json:"tool_id"`
	ServerID     *int64  `json:"server_id"`
	ToolName     string  `json:"tool_name"`
	CallerRole   string  `json:"caller_role"`
	DurationMS   int64   `json:"duration_ms"`
	Status       string  `json:"status" binding:"required"`
	HTTPStatus   int     `json:"http_status"`
	DeniedReason string  `json:"denied_reason"`
	RejectReason string  `json:"reject_reason"`
	CostEstimate float64 `json:"cost_estimate"`
	Parameters   any     `json:"parameters"`
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
	ID             int64           `json:"id" db:"id"`
	ServerID       int64           `json:"server_id" db:"server_id"`
	CategoryID     *int64          `json:"category_id,omitempty" db:"category_id"`
	Name           string          `json:"name" db:"name"`
	Description    string          `json:"description" db:"description"`
	Category       string          `json:"category" db:"category"`
	Tags           []string        `json:"tags" db:"tags"`
	InputSchema    json.RawMessage `json:"input_schema" db:"input_schema"`
	Version        string          `json:"version" db:"version"`
	Published      bool            `json:"published" db:"published"`
	IsSensitive    bool            `json:"is_sensitive" db:"is_sensitive"`
	SensitiveLevel *string         `json:"sensitive_level,omitempty" db:"sensitive_level"`
	HealthStatus   string          `json:"health_status" db:"health_status"`
	CallCount      int64           `json:"call_count" db:"call_count"`
	AverageRating  float64         `json:"average_rating" db:"average_rating"`
	RatingCount    int64           `json:"rating_count" db:"rating_count"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

type ToolCategory struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
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

type ToolPermissionMatrixItem struct {
	Role    string `json:"role" binding:"required"`
	CanRead bool   `json:"can_read"`
	CanCall bool   `json:"can_call"`
}

type ConfigureToolPermissionsRequest struct {
	IsSensitive    bool                       `json:"is_sensitive"`
	SensitiveLevel *string                    `json:"sensitive_level"`
	Permissions    []ToolPermissionMatrixItem `json:"permissions" binding:"required"`
}

type ToolVersion struct {
	ID          int64           `json:"id" db:"id"`
	ToolID      int64           `json:"tool_id" db:"tool_id"`
	Version     string          `json:"version" db:"version"`
	InputSchema json.RawMessage `json:"input_schema" db:"input_schema"`
	Changelog   string          `json:"changelog" db:"changelog"`
	Status      string          `json:"status" db:"status"`
	IsCurrent   bool            `json:"is_current" db:"is_current"`
	ReleasedAt  *time.Time      `json:"released_at,omitempty" db:"released_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

type ToolRating struct {
	ID        int64     `json:"id" db:"id"`
	ToolID    int64     `json:"tool_id" db:"tool_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Rating    int       `json:"rating" db:"rating"`
	Comment   string    `json:"comment" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type ToolRatingSummary struct {
	Average float64 `json:"average"`
	Count   int64   `json:"count"`
}
