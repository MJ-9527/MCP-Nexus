package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// ---------- OpenAPI 最小模型（仅解析支持的子集） ----------

type oaSpec struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths      map[string]json.RawMessage `json:"paths"`
	Security   []map[string][]string      `json:"security"`
	Components struct {
		Schemas       map[string]json.RawMessage `json:"schemas"`
		Parameters    map[string]json.RawMessage `json:"parameters"`
		RequestBodies map[string]json.RawMessage `json:"requestBodies"`
		SecuritySch   map[string]json.RawMessage `json:"securitySchemes"`
	} `json:"components"`
}

type oaPathItem struct {
	Summary     string     `json:"summary"`
	Description string     `json:"description"`
	Get         *oaOp      `json:"get"`
	Post        *oaOp      `json:"post"`
	Put         *oaOp      `json:"put"`    // 出现即报错（不支持）
	Delete      *oaOp      `json:"delete"` // 出现即报错（不支持）
	Parameters  []*oaParam `json:"parameters"`
}

type oaOp struct {
	OperationID string                `json:"operationId"`
	Summary     string                `json:"summary"`
	Description string                `json:"description"`
	Parameters  []*oaParam            `json:"parameters"`
	RequestBody *oaReqBody            `json:"requestBody"`
	Security    []map[string][]string `json:"security"`
}

type oaParam struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Ref         string          `json:"$ref"`
}

type oaReqBody struct {
	Required bool `json:"required"`
	Content  map[string]struct {
		Schema json.RawMessage `json:"schema"`
	} `json:"content"`
	Ref string `json:"$ref"`
}

// ---------- 错误 ----------

// ErrUnsupportedDetail 不支持的 OpenAPI 特性（要求明确报错，禁止静默降级）
type ErrUnsupportedDetail struct{ Detail string }

func (e *ErrUnsupportedDetail) Error() string { return "不支持的 OpenAPI 特性: " + e.Detail }

func unsupportedf(format string, a ...any) error {
	return &ErrUnsupportedDetail{Detail: fmt.Sprintf(format, a...)}
}

// OpenAPIImporter OpenAPI 3.x → MCP 工具骨架生成器（限定子集）
type OpenAPIImporter struct{}

func NewOpenAPIImporter() *OpenAPIImporter { return &OpenAPIImporter{} }

// parsedSpec 解析结果
type parsedSpec struct {
	Title    string
	BaseURL  string
	AuthType string // none / api_key / bearer
	AuthName string // api_key 的 header 名
	Ops      []parsedOp
}

type parsedOp struct {
	ToolName    string
	Description string
	Method      string // GET/POST（上游）
	Path        string // 上游路径模板 /pets/{id}
	InputSchema map[string]any
	// 参数映射信息，供骨架生成参数拆分逻辑
	PathParams   []string
	QueryParams  []string
	HeaderParams []string
	HasBody      bool
	// 生成骨架用的路由键（工具唯一名）
	RouteKey string
}

// Parse 解析并校验 OpenAPI spec（JSON 或 YAML）
func (im *OpenAPIImporter) Parse(specText string) (*parsedSpec, error) {
	specJSON, err := normalizeSpecJSON(specText)
	if err != nil {
		return nil, err
	}
	var spec oaSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return nil, fmt.Errorf("解析 OpenAPI 文档失败: %w", err)
	}
	if !strings.HasPrefix(spec.OpenAPI, "3") {
		return nil, unsupportedf("仅支持 OpenAPI 3.x，当前版本 %q", spec.OpenAPI)
	}

	// 安全方案判定（全局 security + securitySchemes）
	authType, authName := im.resolveAuth(&spec)
	if strings.HasPrefix(authType, "unsupported:") {
		return nil, unsupportedf("安全方案 %s（仅支持 API Key 和 Bearer Token）", strings.TrimPrefix(authType, "unsupported:"))
	}

	// base URL：显式传入优先，否则取第一个 server
	baseURL := ""
	if len(spec.Servers) > 0 {
		baseURL = strings.TrimRight(spec.Servers[0].URL, "/")
	}

	ps := &parsedSpec{Title: spec.Info.Title, BaseURL: baseURL, AuthType: authType, AuthName: authName}

	seenTools := map[string]bool{}
	for path, rawItem := range spec.Paths {
		var item oaPathItem
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, fmt.Errorf("解析 path %s 失败: %w", path, err)
		}
		// 不支持的方法必须明确报错
		if item.Put != nil {
			return nil, unsupportedf("HTTP 方法 PUT（path=%s）", path)
		}
		if item.Delete != nil {
			return nil, unsupportedf("HTTP 方法 DELETE（path=%s）", path)
		}
		for method, op := range map[string]*oaOp{"GET": item.Get, "POST": item.Post} {
			if op == nil {
				continue
			}
			tool, err := im.buildOp(ps, specJSON, path, method, op, item.Parameters)
			if err != nil {
				return nil, err
			}
			if seenTools[tool.ToolName] {
				return nil, fmt.Errorf("operationId %q 重复，工具名必须唯一", tool.ToolName)
			}
			seenTools[tool.ToolName] = true
			ps.Ops = append(ps.Ops, *tool)
		}
	}
	if len(ps.Ops) == 0 {
		return nil, fmt.Errorf("spec 中未发现任何 GET/POST 操作")
	}
	if ps.BaseURL == "" {
		return nil, fmt.Errorf("无法确定上游地址：spec 无 servers，请显式传入 base_url")
	}
	return ps, nil
}

// normalizeSpecJSON：YAML → JSON 归一化
func normalizeSpecJSON(specText string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(specText)
	if trimmed == "" {
		return nil, fmt.Errorf("spec 不能为空")
	}
	if strings.HasPrefix(trimmed, "{") {
		return json.RawMessage(trimmed), nil
	}
	out, err := yaml.YAMLToJSON([]byte(trimmed))
	if err != nil {
		return nil, fmt.Errorf("解析 YAML 失败: %w", err)
	}
	return out, nil
}

// resolveAuth 判定认证方案：仅支持 apiKey(header) 和 http bearer
func (im *OpenAPIImporter) resolveAuth(spec *oaSpec) (string, string) {
	// 收集 operation 级 security（简化：全局/操作取第一个 requirement）
	requirement := map[string][]string{}
	if len(spec.Security) > 0 {
		for k, v := range spec.Security[0] {
			requirement[k] = v
		}
	}
	for name := range requirement {
		raw, ok := spec.Components.SecuritySch[name]
		if !ok {
			continue
		}
		var scheme struct {
			Type   string `json:"type"`
			Scheme string `json:"scheme"`
			In     string `json:"in"`
			Name   string `json:"name"`
		}
		if json.Unmarshal(raw, &scheme) != nil {
			continue
		}
		switch scheme.Type {
		case "http":
			if scheme.Scheme == "" || scheme.Scheme == "bearer" {
				return "bearer", ""
			}
			return "unsupported:" + scheme.Scheme, ""
		case "apiKey":
			if scheme.In != "header" {
				return "unsupported:apiKey(" + scheme.In + ")", ""
			}
			return "api_key", scheme.Name
		case "oauth2", "openIdConnect":
			return "unsupported:" + scheme.Type, ""
		}
	}
	return "none", ""
}

// deref 解析内部 $ref（#/components/...），仅支持文档内引用
func (im *OpenAPIImporter) deref(specJSON json.RawMessage, ref string) (json.RawMessage, error) {
	if ref == "" {
		return nil, nil
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil, unsupportedf("外部 $ref 引用 %q（仅支持文档内 #/ 引用）", ref)
	}
	var root any
	if err := json.Unmarshal(specJSON, &root); err != nil {
		return nil, err
	}
	cur := root
	for _, seg := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		seg = strings.ReplaceAll(seg, "~1", "/")
		seg = strings.ReplaceAll(seg, "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, unsupportedf("$ref %q 解析失败", ref)
		}
		next, ok := m[seg]
		if !ok {
			return nil, fmt.Errorf("$ref %q 指向不存在的内容", ref)
		}
		cur = next
	}
	return json.Marshal(cur)
}

// schemaToProperty 把参数 schema 转成 JSON Schema property（浅校验 + 透传）
func (im *OpenAPIImporter) schemaToProperty(specJSON json.RawMessage, raw json.RawMessage, desc string) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{"type": "string", "description": desc}, nil
	}
	// $ref 解引用
	var probe struct {
		Ref string `json:"$ref"`
	}
	if json.Unmarshal(raw, &probe) == nil && probe.Ref != "" {
		resolved, err := im.deref(specJSON, probe.Ref)
		if err != nil {
			return nil, err
		}
		raw = resolved
	}
	var sch map[string]any
	if err := json.Unmarshal(raw, &sch); err != nil {
		return nil, fmt.Errorf("schema 非法: %w", err)
	}
	// 不支持的组合必须报错，避免生成错误的骨架
	if t, _ := sch["type"].(string); t == "object" && desc == "" {
		// 顶层 object 仅允许出现在 body（由 buildOp 处理），参数级 object 直接透传
	}
	if _, ok := sch["allOf"]; ok {
		return nil, unsupportedf("schema 组合关键字 allOf")
	}
	if _, ok := sch["oneOf"]; ok {
		return nil, unsupportedf("schema 组合关键字 oneOf")
	}
	if _, ok := sch["anyOf"]; ok {
		return nil, unsupportedf("schema 组合关键字 anyOf")
	}
	if desc != "" {
		if existing, _ := sch["description"].(string); existing == "" {
			sch["description"] = desc
		}
	}
	return sch, nil
}

// buildOp 把单个操作转成 MCP 工具定义
func (im *OpenAPIImporter) buildOp(ps *parsedSpec, specJSON json.RawMessage, path, method string, op *oaOp, pathLevelParams []*oaParam) (*parsedOp, error) {
	// 解析参数（$ref → 解引用；path/query/header；cookie 报错）
	props := map[string]any{}
	required := []string{}
	pathParams, queryParams, headerParams := []string{}, []string{}, []string{}

	allParams := append(append([]*oaParam{}, pathLevelParams...), op.Parameters...)
	for _, p := range allParams {
		if p == nil {
			continue
		}
		if p.Ref != "" {
			raw, err := im.deref(specJSON, p.Ref)
			if err != nil {
				return nil, err
			}
			p = new(oaParam)
			if err := json.Unmarshal(raw, p); err != nil {
				return nil, err
			}
		}
		switch p.In {
		case "path", "query", "header":
		case "cookie":
			return nil, unsupportedf("cookie 参数 %q（path=%s）", p.Name, path)
		default:
			return nil, unsupportedf("参数位置 %q（%s %s）", p.In, method, path)
		}
		prop, err := im.schemaToProperty(specJSON, p.Schema, p.Description)
		if err != nil {
			return nil, err
		}
		if _, exists := props[p.Name]; exists {
			return nil, fmt.Errorf("参数 %q 重名（%s %s），请调整 API 定义", p.Name, method, path)
		}
		props[p.Name] = prop
		if p.Required || p.In == "path" {
			required = append(required, p.Name)
		}
		switch p.In {
		case "path":
			pathParams = append(pathParams, p.Name)
		case "query":
			queryParams = append(queryParams, p.Name)
		case "header":
			headerParams = append(headerParams, p.Name)
		}
	}

	// requestBody：仅 application/json
	hasBody := false
	if op.RequestBody != nil {
		rb := op.RequestBody
		if rb.Ref != "" {
			raw, err := im.deref(specJSON, rb.Ref)
			if err != nil {
				return nil, err
			}
			rb = new(oaReqBody)
			if err := json.Unmarshal(raw, rb); err != nil {
				return nil, err
			}
		}
		for ct := range rb.Content {
			if ct != "application/json" {
				return nil, unsupportedf("请求体类型 %q（%s %s），仅支持 application/json", ct, method, path)
			}
		}
		if len(rb.Content) == 0 {
			return nil, unsupportedf("requestBody 缺少 content（%s %s）", method, path)
		}
		bodySchema := rb.Content["application/json"].Schema
		prop, err := im.schemaToProperty(specJSON, bodySchema, "")
		if err != nil {
			return nil, err
		}
		if t, _ := prop["type"].(string); t == "object" {
			// object body 展开合并进顶层参数
			if subProps, ok := prop["properties"].(map[string]any); ok {
				for name, sp := range subProps {
					if _, exists := props[name]; exists {
						return nil, fmt.Errorf("body 字段 %q 与参数重名（%s %s）", name, method, path)
					}
					props[name] = sp
				}
			}
			if subReq, ok := prop["required"].([]any); ok {
				for _, r := range subReq {
					if rs, ok := r.(string); ok {
						required = append(required, rs)
					}
				}
			}
		} else {
			// 非 object body 作为 body 单参数
			props["body"] = prop
			if rb.Required {
				required = append(required, "body")
			}
		}
		hasBody = true
	}

	// 工具名：operationId 优先，缺失则 method_path 生成
	toolName := op.OperationID
	if toolName == "" {
		toolName = strings.ToLower(method) + "_" + strings.NewReplacer("/", "_", "{", "", "}", "", "-", "_").Replace(strings.Trim(path, "/"))
	}
	if len(toolName) > 100 {
		return nil, fmt.Errorf("operationId %q 超过 100 字符", toolName)
	}

	desc := op.Summary
	if op.Description != "" {
		if desc != "" {
			desc += "。"
		}
		desc += op.Description
	}
	if desc == "" {
		desc = fmt.Sprintf("来自 OpenAPI 导入：%s %s", method, path)
	}

	if required == nil {
		required = []string{}
	}
	inputSchema := map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
	return &parsedOp{
		ToolName: toolName, Description: desc,
		Method: method, Path: path, InputSchema: inputSchema,
		PathParams: pathParams, QueryParams: queryParams, HeaderParams: headerParams,
		HasBody: hasBody, RouteKey: toolName,
	}, nil
}

// ---------- 骨架生成 ----------

const skeletonDockerfile = `FROM docker.m.daocloud.io/library/golang:1.26 AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod init mcp-server && go mod tidy
COPY main.go .
RUN CGO_ENABLED=0 go build -o /mcp-server .
FROM docker.m.daocloud.io/library/alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /mcp-server /mcp-server
ENV PORT=9100
EXPOSE 9100
ENTRYPOINT ["/mcp-server"]
`

// GenerateSkeleton 生成的工具定义转成可运行 Go MCP Server 骨架
func (im *OpenAPIImporter) GenerateSkeleton(ps *parsedSpec) (files []GeneratedFileData, envTemplate string) {
	// main.go
	var b strings.Builder
	b.WriteString("// 由 MCP-Nexus OpenAPI 自动包装器生成\n")
	b.WriteString("package main\n\nimport (\n\t\"bytes\"\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n\t\"net/url\"\n\t\"os\"\n\t\"strings\"\n)\n\n")
	b.WriteString("type Tool struct {\n\tName string\n\tDescription string\n\tSchema map[string]any\n\tMethod string\n\tPath string\n\tPathParams []string\n\tQueryParams []string\n\tHeaderParams []string\n\tHasBody bool\n}\n\n")
	b.WriteString("var tools = []Tool{\n")
	for _, op := range ps.Ops {
		schemaJSON, _ := json.Marshal(op.InputSchema)
		b.WriteString(fmt.Sprintf("\t{Name: %q, Description: %q, Schema: mustSchema(%q), Method: %q, Path: %q, PathParams: []string{%s}, QueryParams: []string{%s}, HeaderParams: []string{%s}, HasBody: %v},\n",
			op.ToolName, op.Description, string(schemaJSON), op.Method, op.Path,
			quoteList(op.PathParams), quoteList(op.QueryParams), quoteList(op.HeaderParams),
			strconv.FormatBool(op.HasBody)))
	}
	b.WriteString("}\n\nfunc mustSchema(s string) map[string]any {\n\tvar m map[string]any\n\tif err := json.Unmarshal([]byte(s), &m); err != nil {\n\t\tpanic(err)\n\t}\n\treturn m\n}\n\n")

	// main 函数：/health + /tools/{name}/call，参数拆分转发上游
	b.WriteString("func main() {\n\tbaseURL := os.Getenv(\"UPSTREAM_BASE_URL\")\n\tauthHeader := os.Getenv(\"AUTH_HEADER\")\n\tmux := http.NewServeMux()\n\tmux.HandleFunc(\"/health\", func(w http.ResponseWriter, r *http.Request) {\n\t\tw.Header().Set(\"Content-Type\", \"application/json\")\n\t\tfmt.Fprint(w, `{\"status\":\"ok\"}`)\n\t})\n")
	b.WriteString("\tmux.HandleFunc(\"/tools/\", func(w http.ResponseWriter, r *http.Request) {\n")
	b.WriteString("\t\tname := strings.Trim(strings.TrimPrefix(r.URL.Path, \"/tools/\"), \"/\")\n")
	b.WriteString("\t\tif !strings.HasSuffix(name, \"/call\") {\n\t\t\thttp.NotFound(w, r); return\n\t\t}\n")
	b.WriteString("\t\tname = strings.TrimSuffix(name, \"/call\")\n")
	b.WriteString("\t\tvar tool *Tool\n\t\tfor i := range tools {\n\t\t\tif tools[i].Name == name { tool = &tools[i] }\n\t\t}\n")
	b.WriteString("\t\tif tool == nil { http.NotFound(w, r); return }\n\n")
	b.WriteString("\t\tvar args map[string]any\n\t\t_ = json.NewDecoder(r.Body).Decode(&args)\n\t\tif args == nil { args = map[string]any{} }\n\n")
	b.WriteString("\t\tpath := tool.Path\n\t\tq := url.Values{}\n\t\tfor _, p := range tool.PathParams {\n\t\t\tv, _ := args[p].(string)\n\t\t\tpath = strings.ReplaceAll(path, \"{\"+p+\"}\", v)\n\t\t}\n")
	b.WriteString("\t\tfor _, p := range tool.QueryParams { if v, ok := args[p]; ok { q.Set(p, fmt.Sprintf(\"%v\", v)) } }\n")
	b.WriteString("\t\tif len(q) > 0 { path += \"?\" + q.Encode() }\n")
	b.WriteString("\t\tbody := map[string]any{}\n\t\tif tool.HasBody { for k, v := range args { if !contains(tool.PathParams, k) && !contains(tool.QueryParams, k) && !contains(tool.HeaderParams, k) { body[k] = v } } }\n")
	b.WriteString("\t\tvar req *http.Request\n\t\tif tool.HasBody {\n\t\t\tbs, _ := json.Marshal(body)\n\t\t\treq, _ = http.NewRequest(tool.Method, baseURL+path, bytes.NewReader(bs))\n\t\t\treq.Header.Set(\"Content-Type\", \"application/json\")\n\t\t} else {\n\t\t\treq, _ = http.NewRequest(tool.Method, baseURL+path, nil)\n\t\t}\n")
	b.WriteString("\t\tfor _, p := range tool.HeaderParams { if v, ok := args[p]; ok { req.Header.Set(p, fmt.Sprintf(\"%v\", v)) } }\n")
	b.WriteString("\t\tif authHeader != \"\" { req.Header.Set(\"Authorization\", authHeader) }\n")
	b.WriteString("\t\tresp, err := http.DefaultClient.Do(req)\n\t\tif err != nil { w.WriteHeader(502); fmt.Fprint(w, err.Error()); return }\n")
	b.WriteString("\t\tdefer resp.Body.Close()\n\t\traw, _ := io.ReadAll(resp.Body)\n\t\tw.Header().Set(\"Content-Type\", \"application/json\")\n\t\tw.WriteHeader(resp.StatusCode)\n\t\tw.Write(raw)\n\t})\n")
	b.WriteString("\tport := os.Getenv(\"PORT\")\n\tif port == \"\" { port = \"9100\" }\n\tfmt.Println(\"MCP skeleton server listening on :\" + port)\n\thttp.ListenAndServe(\":\"+port, mux)\n}\n\n")
	b.WriteString("func contains(xs []string, x string) bool { for _, v := range xs { if v == x { return true } }; return false }\n")

	files = append(files, GeneratedFileData{Path: "main.go", Content: b.String()})
	files = append(files, GeneratedFileData{Path: "Dockerfile", Content: skeletonDockerfile})

	// env 模板
	env := "UPSTREAM_BASE_URL=" + ps.BaseURL + "\nPORT=9100\n"
	switch ps.AuthType {
	case "api_key":
		env += "# API Key 以 header 传递时在骨架中补一行：req.Header.Set(\"" + ps.AuthName + "\", os.Getenv(\"API_KEY\"))\nAPI_KEY=\n"
		env += "AUTH_HEADER=Bearer \n"
	case "bearer":
		env += "AUTH_HEADER=Bearer <your-token>\n"
	default:
		env += "AUTH_HEADER=\n"
	}
	files = append(files, GeneratedFileData{Path: ".env.example", Content: env})
	return files, env
}

// GeneratedFileData 生成的文件（service 层内部用）
type GeneratedFileData struct {
	Path    string
	Content string
}

func quoteList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, strconv.Quote(x))
	}
	return strings.Join(parts, ", ")
}
