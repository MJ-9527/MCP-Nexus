package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
)

// SkillsTranslator 将 MCP 工具调用翻译为上游 Skills 服务的实际 HTTP 请求。
// 与 OpenAPITranslator 工作流程一致，差异仅在 baseURL 来源：
//   - OpenAPITranslator：baseURL = server.Endpoint（同一 Server 一个上游 API）
//   - SkillsTranslator：baseURL = spec.Endpoint（per-tool 调用地址，空则 fallback server.Endpoint）
//
// 工作流程：
//  1. 从 spec.PathTemplate 用 arguments 中的 path 参数填充 {占位符}
//  2. 将 spec.Params.Query 列出的参数拼到 query string
//  3. 若 spec.Params.Body 非空，将 arguments[Body] 作为 JSON 请求体发送
//  4. 发送 HTTP 请求到 spec.Endpoint（或 server.Endpoint） + 填充后的 path
//  5. 上游响应体包装为 McpToolCallResponse（成功/失败统一结构）
type SkillsTranslator struct {
	httpClient *client.MCPClient
}

func NewSkillsTranslator(httpClient *client.MCPClient) *SkillsTranslator {
	return &SkillsTranslator{httpClient: httpClient}
}

// Translate 执行翻译与调用：构造请求 → 发送到上游 Skills 服务 → 包装响应。
// requestID 用于透传到上游（与原生 MCP / OpenAPI 调用一致，便于链路追踪）。
func (t *SkillsTranslator) Translate(ctx context.Context, spec *model.SkillsSpec, server *model.MCPServer, arguments map[string]interface{}, requestID string) (*model.McpToolCallResponse, error) {
	var params model.SkillsParams
	if len(spec.Params) > 0 {
		if err := json.Unmarshal(spec.Params, &params); err != nil {
			return nil, fmt.Errorf("%w: parse skills params: %v", ErrUpstreamError, err)
		}
	}
	// baseURL 优先取 spec.Endpoint，空则 fallback server.Endpoint
	baseURL := strings.TrimSpace(spec.Endpoint)
	if baseURL == "" {
		baseURL = server.Endpoint
	}
	// 复制一份 arguments，避免修改调用方原始 map
	args := copyArgs(arguments)
	fullURL, body, err := buildSkillsRequest(baseURL, spec.PathTemplate, params, args)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"request_id": requestID}
	respBody, status, err := t.httpClient.DoRequest(ctx, spec.Method, fullURL, body, headers)
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	return wrapSkillsResponse(status, respBody), nil
}

// buildSkillsRequest 构造目标 URL 与请求体：path 参数填充 path template，query 拼到 query string，
// body 参数作为 JSON 请求体。path 参数缺失返回错误（避免发出带 {占位符} 的请求）。
func buildSkillsRequest(baseURL, pathTemplate string, params model.SkillsParams, args map[string]interface{}) (string, []byte, error) {
	path := pathTemplate
	// path 参数：替换 {name} 占位符
	for _, p := range params.Path {
		raw, ok := args[p]
		if !ok {
			return "", nil, fmt.Errorf("%w: missing path parameter %q", ErrUpstreamError, p)
		}
		delete(args, p)
		placeholder := "{" + p + "}"
		path = strings.ReplaceAll(path, placeholder, url.PathEscape(fmt.Sprintf("%v", raw)))
	}
	// 仍存在 {xxx} 占位符（未在 params.Path 中声明）：尝试用 args 同名值替换
	// 适配默认 path_template=/tools/{name}/call 这类无 params.Path 声明的场景
	if strings.Contains(path, "{") {
		path = replaceUnmatchedPathPlaceholders(path, args)
	}
	if strings.Contains(path, "{") {
		return "", nil, fmt.Errorf("%w: unresolved path placeholders in %q", ErrUpstreamError, path)
	}
	// query 参数
	q := url.Values{}
	for _, p := range params.Query {
		if v, ok := args[p]; ok {
			q.Set(p, fmt.Sprintf("%v", v))
			delete(args, p)
		}
	}
	// body 参数
	var body []byte
	if params.Body != "" {
		if v, ok := args[params.Body]; ok {
			b, err := json.Marshal(v)
			if err != nil {
				return "", nil, fmt.Errorf("%w: marshal body: %v", ErrUpstreamError, err)
			}
			body = b
			delete(args, params.Body)
		}
	}
	// 构造完整 URL
	fullURL := strings.TrimRight(baseURL, "/") + path
	if len(q) > 0 {
		fullURL += "?" + q.Encode()
	}
	return fullURL, body, nil
}

// replaceUnmatchedPathPlaceholders 替换 path 中未被 params.Path 显式声明的 {占位符}。
// 用 args 同名值替换，找不到的占位符保留不动（由上层报错）。
// 用于默认 path_template /tools/{name}/call 与 arguments 中无对应 params.Path 声明的场景。
func replaceUnmatchedPathPlaceholders(path string, args map[string]interface{}) string {
	for name, val := range args {
		placeholder := "{" + name + "}"
		if strings.Contains(path, placeholder) {
			path = strings.ReplaceAll(path, placeholder, url.PathEscape(fmt.Sprintf("%v", val)))
		}
	}
	return path
}

// wrapSkillsResponse 将上游 HTTP 响应包装为统一的 MCP 响应结构。
// 非 2xx 视为错误（IsError=true），但仍返回响应体便于调用方诊断。
func wrapSkillsResponse(status int, body []byte) *model.McpToolCallResponse {
	isError := status < 200 || status > 299
	text := string(body)
	if len(text) == 0 {
		text = http.StatusText(status)
	}
	return &model.McpToolCallResponse{
		Content: []map[string]interface{}{
			{"type": "text", "text": text},
		},
		IsError: isError,
	}
}
