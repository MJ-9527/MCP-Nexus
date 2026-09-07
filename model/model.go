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
	Status       string    `json:"status" db:"status"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
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
type ToolVersion struct {
	ID          int64           `json:"id" db:"id"`
	ToolID      int64           `json:"tool_id" db:"tool_id"`
	Version     string          `json:"version" db:"version"`
	InputSchema json.RawMessage `json:"input_schema" db:"input_schema"`
	Changelog   string          `json:"changelog" db:"changelog"`
	Status      string          `json:"status" db:"status"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}
