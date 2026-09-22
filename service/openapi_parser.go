package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"MCP-Nexus/model"
)

// OpenAPIOperation 解析 spec 得到的单个 operation 中间结构，供导入器写入 tool + spec。
type OpenAPIOperation struct {
	OperationID string
	Summary     string
	Method      string
	Path        string
	Params      model.OpenAPIParams
	InputSchema json.RawMessage
}

// OpenAPIParser 解析 OpenAPI/Swagger JSON 规范，将每个 path+method 翻译为 OpenAPIOperation。
// 支持 OpenAPI 3.x 与 Swagger 2.0：二者 paths 结构基本一致，主要差异在 requestBody 字段
// （Swagger 用 parameters/in=body 表示请求体），此处对两种写法均做识别。
type OpenAPIParser struct{}

func NewOpenAPIParser() *OpenAPIParser { return &OpenAPIParser{} }

// Parse 解析 spec JSON，返回所有 operation。空规范或缺少 paths 视为非法。
func (p *OpenAPIParser) Parse(spec json.RawMessage) ([]OpenAPIOperation, error) {
	if len(spec) == 0 || !json.Valid(spec) {
		return nil, ErrInvalidOpenAPI
	}
	var doc struct {
		OpenAPI string                                `json:"openapi"`
		Swagger string                                `json:"swagger"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOpenAPI, err)
	}
	if doc.OpenAPI == "" && doc.Swagger == "" {
		return nil, fmt.Errorf("%w: missing openapi/swagger version", ErrInvalidOpenAPI)
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("%w: no paths", ErrInvalidOpenAPI)
	}
	ops := make([]OpenAPIOperation, 0)
	// 按 path+method 稳定顺序输出，便于测试断言与幂等对比
	paths := sortedPathKeys(doc.Paths)
	for _, path := range paths {
		methods := doc.Paths[path]
		for _, method := range sortedMethodKeys(methods) {
			rawOp := methods[method]
			upMethod := strings.ToUpper(method)
			if !isHTTPMethod(upMethod) {
				continue // 跳过 summary/parameters/servers 等非 operation 字段
			}
			op, err := parseOperation(upMethod, path, rawOp)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}

func parseOperation(method, path string, raw json.RawMessage) (OpenAPIOperation, error) {
	var op struct {
		OperationID string `json:"operationId"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Parameters  []struct {
			Name     string          `json:"name"`
			In       string          `json:"in"`
			Required bool            `json:"required"`
			Schema   json.RawMessage `json:"schema"`
		} `json:"parameters"`
		RequestBody struct {
			Content map[string]struct {
				Schema json.RawMessage `json:"schema"`
			} `json:"content"`
		} `json:"requestBody"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return OpenAPIOperation{}, fmt.Errorf("%w: parse operation %s %s: %v", ErrInvalidOpenAPI, method, path, err)
	}
	name := op.OperationID
	if name == "" {
		name = defaultOperationName(method, path)
	}
	summary := op.Summary
	if summary == "" {
		summary = op.Description
	}
	params := model.OpenAPIParams{Path: []string{}, Query: []string{}}
	schemaProps := map[string]json.RawMessage{}
	propRequired := map[string]bool{}
	for _, p := range op.Parameters {
		switch strings.ToLower(p.In) {
		case "path":
			params.Path = append(params.Path, p.Name)
			schemaProps[p.Name] = p.Schema
			if p.Required {
				propRequired[p.Name] = true
			}
		case "query":
			params.Query = append(params.Query, p.Name)
			schemaProps[p.Name] = p.Schema
			if p.Required {
				propRequired[p.Name] = true
			}
		case "header", "cookie":
			// header/cookie 参数不作为工具入参暴露，避免与网关自身 Header 冲突
		case "body":
			// Swagger 2.0 的 in=body 参数：整个 schema 作为请求体
			params.Body = "body"
			if len(p.Schema) > 0 {
				schemaProps["body"] = p.Schema
			}
		}
	}
	// OpenAPI 3.x 的 requestBody.application/json
	if bodySchema, ok := op.RequestBody.Content["application/json"]; ok && len(bodySchema.Schema) > 0 {
		params.Body = "body"
		schemaProps["body"] = bodySchema.Schema
	}
	inputSchema := buildInputSchema(schemaProps, propRequired)
	return OpenAPIOperation{
		OperationID: name,
		Summary:     summary,
		Method:      method,
		Path:        path,
		Params:      params,
		InputSchema: inputSchema,
	}, nil
}

// buildInputSchema 由 operation 参数生成 MCP 工具的 input_schema（JSON Schema object）。
// 缺失子 schema 时退化为 {"type":"string"}，保证结构合法可发布。
func buildInputSchema(props map[string]json.RawMessage, required map[string]bool) json.RawMessage {
	if len(props) == 0 {
		return []byte(`{"type":"object","properties":{}}`)
	}
	propObj := map[string]json.RawMessage{}
	reqList := make([]string, 0)
	for k, v := range props {
		if len(v) == 0 || string(v) == "null" {
			propObj[k] = []byte(`{"type":"string"}`)
		} else {
			propObj[k] = v
		}
		if required[k] {
			reqList = append(reqList, k)
		}
	}
	// 序列化 properties 子对象
	propsBytes, _ := json.Marshal(propObj)
	// 构造外层 schema
	out := map[string]any{
		"type":       "object",
		"properties": json.RawMessage(propsBytes),
	}
	if len(reqList) > 0 {
		out["required"] = reqList
	}
	b, _ := json.Marshal(out)
	return b
}

// defaultOperationName 在 operationId 缺失时用 method+path 生成工具名。
// 将 / 与 { } 等非字母数字字符折叠为单下划线并去首尾下划线，避免出现连续下划线。
func defaultOperationName(method, path string) string {
	var b strings.Builder
	prevUnderscore := true // 起始视作已加下划线，避免头部多余下划线
	flush := func(r rune) {
		if r == 0 {
			return
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	for _, r := range strings.Trim(path, "/") {
		flush(r)
	}
	name := strings.ToLower(method) + "_" + strings.Trim(b.String(), "_")
	return name
}

func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func sortedPathKeys(m map[string]map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 稳定排序
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

func sortedMethodKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
