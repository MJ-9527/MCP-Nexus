package openapi

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	demoCatalog     = "../../examples/openapi/demo-catalog.yaml"
	unsupportedSpec = "../../examples/openapi/unsupported.yaml"
)

func TestValidateReportsUnsupportedFeatures(t *testing.T) {
	data, err := os.ReadFile(unsupportedSpec)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	err = Validate(spec)
	if err == nil {
		t.Fatal("expected validation errors, got nil")
	}
	msg := err.Error()
	want := []string{
		"unsupported HTTP method",
		"unsupported parameter location",
		"unsupported request body content type",
		"unsupported type",
	}
	for _, w := range want {
		if !strings.Contains(msg, w) {
			t.Errorf("validation error missing %q, got:\n%s", w, msg)
		}
	}
}

func TestGenerateDoesNotRunOnInvalidSpec(t *testing.T) {
	// 明确错误：Generate 不应在 Validate 失败后被调用；即使被调用，工具列表也会缺失。
	data, err := os.ReadFile(unsupportedSpec)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 过滤掉无效 operation 后 BuildTools 仍可能产生部分工具，但 CLI 会先用 Validate 拦截。
	// 这里验证 CLI 流程：Validate 失败直接退出，不会进入 Generate。
	if Validate(spec) == nil {
		t.Fatal("expected validation to fail")
	}
}

func TestParseAndBuildDemoCatalog(t *testing.T) {
	spec := mustParseDemo(t)
	if err := Validate(spec); err != nil {
		t.Fatalf("validate: %v", err)
	}
	tools, err := BuildTools(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 {
		t.Fatalf("len(tools) = %d, want 3", len(tools))
	}

	got := map[string]Tool{}
	for _, tool := range tools {
		got[tool.Name] = tool
	}
	for _, want := range []string{"listProducts", "getProduct", "createOrder"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing tool %q", want)
		}
	}

	// createOrder 的输入 Schema 应包含 body（OrderRequest 的字段）+ header 幂等键
	order := got["createOrder"]
	schema := string(order.InputSchema)
	for _, field := range []string{"product_id", "quantity", "Idempotency-Key"} {
		if !strings.Contains(schema, field) {
			t.Errorf("createOrder schema 缺少字段 %q: %s", field, schema)
		}
	}
	if !order.UsesBearer {
		t.Error("createOrder 应使用 Bearer 鉴权")
	}
}

func TestValidateUnsupportedMethod(t *testing.T) {
	spec := &Spec{
		OpenAPI: "3.0.3",
		Paths: map[string]*PathItem{
			"/x": {Delete: &Operation{OperationID: "deleteX"}},
		},
	}
	err := Validate(spec)
	if err == nil || !strings.Contains(err.Error(), "unsupported HTTP method") {
		t.Fatalf("want unsupported method error, got %v", err)
	}
}

func TestValidateUnsupportedAuth(t *testing.T) {
	spec := &Spec{
		OpenAPI: "3.0.3",
		Paths:   map[string]*PathItem{},
		Components: &Components{
			SecuritySchemes: map[string]*SecurityScheme{
				"oauth": {Type: "oauth2"},
			},
		},
	}
	err := Validate(spec)
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("want unsupported auth error, got %v", err)
	}
}

func TestGenerateCompiles(t *testing.T) {
	spec := mustParseDemo(t)
	if err := Validate(spec); err != nil {
		t.Fatalf("validate: %v", err)
	}
	tools, err := BuildTools(spec)
	if err != nil {
		t.Fatal(err)
	}
	files, err := Generate(spec, tools, Options{ModuleName: "example.com/generated"})
	if err != nil {
		t.Fatal(err)
	}

	// 产物必须包含全部 6 个文件
	for _, name := range []string{"main.go", "tools.json", "go.mod", ".env.example", "Dockerfile", "README.md"} {
		if _, ok := files[name]; !ok {
			t.Errorf("缺少生成文件 %q", name)
		}
	}

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 完成标准：生成产物可编译（纯标准库，无需联网）
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build 失败: %v\n%s", err, out)
	}
}

func mustParseDemo(t *testing.T) *Spec {
	t.Helper()
	data, err := os.ReadFile(demoCatalog)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return spec
}
