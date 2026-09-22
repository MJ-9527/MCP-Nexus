package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 一次 operation 转换出的 MCP 工具元数据。
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	InputSchema json.RawMessage `json:"inputSchema"`
	UsesAPIKey  bool            `json:"-"`
	UsesBearer  bool            `json:"-"`
}

// 控制生成行为。
type Options struct {
	ModuleName string // 生成项目的 go module 名
}

// 将 Spec 的每个 GET/POST operation 转成 MCP 工具（按 path 稳定排序）。
func BuildTools(spec *Spec) ([]Tool, error) {
	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var tools []Tool
	for _, p := range paths {
		item := spec.Paths[p]
		for _, m := range []struct {
			method string
			op     *Operation
		}{{"GET", item.Get}, {"POST", item.Post}} {
			if m.op == nil {
				continue
			}
			tools = append(tools, buildTool(spec, p, m.method, m.op))
		}
	}
	return tools, nil
}

func buildTool(spec *Spec, path, method string, op *Operation) Tool {
	desc := op.Summary
	if desc == "" {
		desc = op.Description
	}
	if desc == "" {
		desc = fmt.Sprintf("%s %s", method, path)
	}
	t := Tool{
		Name:        op.OperationID,
		Description: desc,
		Method:      method,
		Path:        path,
		InputSchema: buildInputSchema(spec, op),
	}
	t.UsesAPIKey, t.UsesBearer = usesSecurity(spec, op)
	return t
}

// 将 path/query/header/body 参数合并为一个 JSON Schema。
func buildInputSchema(spec *Spec, op *Operation) json.RawMessage {
	props := map[string]any{}
	var required []string

	for _, p := range op.Parameters {
		if p.Schema == nil {
			continue
		}
		props[p.Name] = schemaToJSON(spec, p.Schema)
		if p.Required {
			required = append(required, p.Name)
		}
	}

	if op.RequestBody != nil {
		if mt, ok := op.RequestBody.Content["application/json"]; ok && mt.Schema != nil {
			rs := resolveSchema(spec, mt.Schema)
			if rs != nil && rs.Type == "object" && rs.Properties != nil {
				for k, v := range rs.Properties {
					props[k] = schemaToJSON(spec, v)
				}
				required = append(required, rs.Required...)
			} else {
				props["body"] = schemaToJSON(spec, rs)
				if op.RequestBody.Required {
					required = append(required, "body")
				}
			}
		}
	}

	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = dedupeStrings(required)
	}
	b, _ := json.Marshal(schema)
	return b
}

// *Schema 转成 JSON Schema map（解析 $ref）。
func schemaToJSON(spec *Spec, s *Schema) map[string]any {
	s = resolveSchema(spec, s)
	out := map[string]any{}
	if s == nil {
		return out
	}
	if s.Type != "" {
		out["type"] = s.Type
	}
	if s.Format != "" {
		out["format"] = s.Format
	}
	if s.Description != "" {
		out["description"] = s.Description
	}
	if len(s.Required) > 0 {
		out["required"] = s.Required
	}
	if s.Properties != nil {
		props := map[string]any{}
		for k, v := range s.Properties {
			props[k] = schemaToJSON(spec, v)
		}
		out["properties"] = props
	}
	if s.Items != nil {
		out["items"] = schemaToJSON(spec, s.Items)
	}
	if s.Default != nil {
		out["default"] = s.Default
	}
	if s.Minimum != nil {
		out["minimum"] = *s.Minimum
	}
	if s.Maximum != nil {
		out["maximum"] = *s.Maximum
	}
	if s.Enum != nil {
		out["enum"] = s.Enum
	}
	return out
}

// usesSecurity 判断 operation 使用了哪些鉴权（operation 级优先，否则用 spec 级）。
func usesSecurity(spec *Spec, op *Operation) (apiKey, bearer bool) {
	reqs := op.Security
	if len(reqs) == 0 {
		reqs = spec.Security
	}
	if spec.Components == nil {
		return false, false
	}
	for _, req := range reqs {
		for scheme := range req {
			ss, ok := spec.Components.SecuritySchemes[scheme]
			if !ok {
				continue
			}
			switch {
			case ss.Type == "apiKey":
				apiKey = true
			case ss.Type == "http" && ss.Scheme == "bearer":
				bearer = true
			}
		}
	}
	return apiKey, bearer
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ---------------------------------------------------------------------------
// 生成产物
// ---------------------------------------------------------------------------

// Generate 产出生成文件的集合：main.go、tools.json、go.mod、.env.example、Dockerfile、README.md。
func Generate(spec *Spec, tools []Tool, opts Options) (map[string][]byte, error) {
	moduleName := opts.ModuleName
	if moduleName == "" {
		moduleName = "generated-server"
	}

	files := map[string][]byte{}

	toolsJSON, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal tools: %w", err)
	}
	files["tools.json"] = append(toolsJSON, '\n')
	files["main.go"] = []byte(generatedMainGo)
	files["go.mod"] = []byte(fmt.Sprintf("module %s\n\ngo 1.26\n", moduleName))
	files[".env.example"] = buildEnvExample(spec, tools)
	files["Dockerfile"] = []byte(generatedDockerfile)
	files["README.md"] = buildREADME(spec, tools, moduleName)

	return files, nil
}

func buildEnvExample(spec *Spec, tools []Tool) []byte {
	var b strings.Builder
	b.WriteString("# 由 openapi-gen 生成的环境变量模板\n\n")

	baseURL := ""
	if len(spec.Servers) > 0 {
		baseURL = spec.Servers[0].URL
	}
	b.WriteString("# 上游服务地址（OpenAPI servers[0].url）\n")
	b.WriteString("BASE_URL=" + baseURL + "\n\n")
	b.WriteString("# 服务监听端口\nPORT=8080\n\n")

	apiKey, bearer := false, false
	for _, t := range tools {
		apiKey = apiKey || t.UsesAPIKey
		bearer = bearer || t.UsesBearer
	}
	if apiKey {
		b.WriteString("# API Key（工具使用 apiKey 鉴权时填写）\nAPI_KEY=\n\n")
	}
	if bearer {
		b.WriteString("# Bearer Token（工具使用 bearer 鉴权时填写）\nBEARER_TOKEN=\n\n")
	}
	return []byte(b.String())
}

func buildREADME(spec *Spec, tools []Tool, moduleName string) []byte {
	var b strings.Builder
	title := spec.Info.Title
	if title == "" {
		title = moduleName
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "由 `openapi-gen` 从 OpenAPI 文档生成的 MCP Server 骨架（version %s）。\n\n", spec.Info.Version)

	b.WriteString("## 启动\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# 本地运行（Go 1.26+）\n")
	b.WriteString("go run .\n\n")
	b.WriteString("# 或容器运行\n")
	b.WriteString("docker build -t generated-server .\n")
	b.WriteString("docker run --rm -p 8080:8080 generated-server\n")
	b.WriteString("```\n\n")
	b.WriteString("服务默认监听 `:8080`，健康检查：`GET /health`。\n\n")

	b.WriteString("## 工具列表\n\n")
	b.WriteString("| 工具名 | 描述 | 上游 |\n|---|---|---|\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "| `%s` | %s | `%s %s` |\n", t.Name, t.Description, t.Method, t.Path)
	}

	b.WriteString("\n## 输入 Schema\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "### %s\n\n```json\n%s\n```\n\n", t.Name, string(t.InputSchema))
	}

	b.WriteString("## 调用\n\n")
	b.WriteString("```bash\n")
	b.WriteString("curl -X POST http://localhost:8080/tools/<tool>/call \\\n")
	b.WriteString("  -H 'Content-Type: application/json' \\\n")
	b.WriteString("  -d '{\"<param>\":\"<value>\"}'\n")
	b.WriteString("```\n")
	return []byte(b.String())
}

// generatedMainGo 是生成骨架的 main.go（纯标准库，无第三方依赖）。
const generatedMainGo = `// 由 openapi-gen 生成的 MCP Server 骨架。
// 仅使用标准库，可直接 ` + "`go run .`" + ` 启动。
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed tools.json
var toolsJSON []byte

type toolSpec struct {
	Name        string          ` + "`json:\"name\"`" + `
	Description string          ` + "`json:\"description\"`" + `
	Method      string          ` + "`json:\"method\"`" + `
	Path        string          ` + "`json:\"path\"`" + `
	InputSchema json.RawMessage ` + "`json:\"inputSchema\"`" + `
}

var tools = loadTools()

func loadTools() map[string]toolSpec {
	var list []toolSpec
	if err := json.Unmarshal(toolsJSON, &list); err != nil {
		log.Fatalf("加载 tools.json 失败: %v", err)
	}
	m := make(map[string]toolSpec, len(list))
	for _, t := range list {
		m[t.Name] = t
	}
	return m
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/tools/", handleTool)
	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/tools/")
	name = strings.TrimSuffix(name, "/call")
	spec, ok := tools[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "tool not found"})
		return
	}
	var args map[string]any
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
	}
	// TODO: 在此实现上游调用 —— 使用 BASE_URL + 鉴权 Header（API_KEY / BEARER_TOKEN），
	// 按 spec.Method / spec.Path 转发，参数见 tools.json 的 inputSchema。
	writeJSON(w, http.StatusOK, map[string]any{
		"tool":     spec.Name,
		"args":     args,
		"upstream": fmt.Sprintf("%s %s", spec.Method, spec.Path),
		"note":     "skeleton: implement upstream call",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
`

const generatedDockerfile = `FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go tools.json ./
RUN go build -o /out/server .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
`
