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

// OpenAPITranslator 将 MCP 工具调用翻译为上游 OpenAPI 服务的实际 HTTP 请求。
// 工作流程：
//  1. 从 spec.PathTemplate 用 arguments 中的 path 参数填充 {占位符}
//  2. 将 spec.Params.Query 列出的参数拼到 query string
//  3. 若 spec.Params.Body 非空，将 arguments[Body] 作为 JSON 请求体发送
//  4. 发送 HTTP 请求到 server.Endpoint + 填充后的 path
//  5. 上游响应体包装为 McpToolCallResponse（成功/失败统一结构）
type OpenAPITranslator struct {
	httpClient *client.MCPClient
}

func NewOpenAPITranslator(httpClient *client.MCPClient) *OpenAPITranslator {
	return &OpenAPITranslator{httpClient: httpClient}
}

// Translate 执行翻译与调用：构造请求 → 发送到上游 → 包装响应。
// requestID 用于透传到上游（与原生 MCP 调用一致，便于链路追踪）。
func (t *OpenAPITranslator) Translate(ctx context.Context, spec *model.OpenAPISpec, server *model.MCPServer, arguments map[string]interface{}, requestID string) (*model.McpToolCallResponse, error) {
	var params model.OpenAPIParams
	if len(spec.Params) > 0 {
		if err := json.Unmarshal(spec.Params, &params); err != nil {
			return nil, fmt.Errorf("%w: parse openapi params: %v", ErrUpstreamError, err)
		}
	}
	// 复制一份 arguments，避免修改调用方原始 map
	args := copyArgs(arguments)
	fullURL, body, err := buildOpenAPIRequest(server.Endpoint, spec.PathTemplate, params, args)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"request_id": requestID}
	respBody, status, err := t.httpClient.DoRequest(ctx, spec.Method, fullURL, body, headers)
	if err != nil {
		return nil, mapUpstreamError(err)
	}
	return wrapOpenAPIResponse(status, respBody), nil
}

// buildOpenAPIRequest 构造目标 URL 与请求体：path 参数填充 path template，query 拼到 query string，
// body 参数作为 JSON 请求体。path 参数缺失返回错误（避免发出带 {占位符} 的请求）。
func buildOpenAPIRequest(baseURL, pathTemplate string, params model.OpenAPIParams, args map[string]interface{}) (string, []byte, error) {
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
	if strings.Contains(path, "{") {
		// 仍存在未替换的占位符（可能是 spec 解析遗漏）
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

// wrapOpenAPIResponse 将上游 HTTP 响应包装为统一的 MCP 响应结构。
// 非 2xx 视为错误（IsError=true），但仍返回响应体便于调用方诊断。
func wrapOpenAPIResponse(status int, body []byte) *model.McpToolCallResponse {
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

// copyArgs 深拷贝 arguments map，避免修改调用方原始数据。
func copyArgs(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
