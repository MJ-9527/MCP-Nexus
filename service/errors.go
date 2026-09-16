package service

import "errors"

// 网关业务错误（B4）。ErrToolNotFound / ErrServerNotFound 定义于 tool_register.go，
// ErrPermissionDenied 定义于 tool_permission.go，此处补充调用链新增的错误。
// handler 层据此映射统一错误码与 HTTP 状态码（见 handler/proxy_handle.go）。
var (
	// ErrToolOffline 工具或其所属服务已下线
	ErrToolOffline = errors.New("tool offline")
	// ErrServerUnavailable MCP Server 不可用（未激活 / 离线 / 连接失败）
	ErrServerUnavailable = errors.New("server unavailable")
	// ErrUpstreamTimeout 上游调用超时
	ErrUpstreamTimeout = errors.New("upstream timeout")
	// ErrUpstreamError 上游返回错误（非 2xx / 响应非法 / 响应体超限）
	ErrUpstreamError = errors.New("upstream error")
)
