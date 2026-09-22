package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"MCP-Nexus/model"
)

// B12 校验器单测：覆盖必填缺失、类型不匹配、版本不匹配、空 schema 放行、
// 无 required 放行、额外字段放行、版本缺省放行等关键场景。

// schema 构造辅助
func mustJSON(t *testing.T, v string) json.RawMessage {
	t.Helper()
	if !json.Valid([]byte(v)) {
		t.Fatalf("invalid json: %s", v)
	}
	return json.RawMessage(v)
}

func TestValidateArguments_NilSchemaPasses(t *testing.T) {
	// schema 为空：无法可依，放行
	if err := ValidateArguments(nil, map[string]interface{}{"a": 1}); err != nil {
		t.Fatalf("空 schema 应放行，实际 %v", err)
	}
	if err := ValidateArguments(json.RawMessage(`null`), map[string]interface{}{}); err != nil {
		t.Fatalf("null schema 应放行，实际 %v", err)
	}
}

func TestValidateArguments_InvalidSchemaPasses(t *testing.T) {
	// schema 自身非法：放行，避免导入阶段的脏数据阻塞调用
	if err := ValidateArguments(json.RawMessage(`{not-json}`), map[string]interface{}{}); err != nil {
		t.Fatalf("非法 schema 应放行，实际 %v", err)
	}
}

func TestValidateArguments_NonObjectSchemaPasses(t *testing.T) {
	// type 非 object 不在校验器范围
	schema := mustJSON(t, `{"type":"array","items":{"type":"string"}}`)
	if err := ValidateArguments(schema, map[string]interface{}{}); err != nil {
		t.Fatalf("非 object schema 应放行，实际 %v", err)
	}
}

func TestValidateArguments_NoRequiredPasses(t *testing.T) {
	// 声明 properties 但无 required：未传字段不报错
	schema := mustJSON(t, `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}}}`)
	if err := ValidateArguments(schema, map[string]interface{}{}); err != nil {
		t.Fatalf("无 required 时应放行，实际 %v", err)
	}
}

func TestValidateArguments_RequiredMissing(t *testing.T) {
	schema := mustJSON(t, `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"]}`)
	err := ValidateArguments(schema, map[string]interface{}{"name": "alice"})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("期望 ErrInvalidArguments，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "age") {
		t.Fatalf("错误信息应包含缺失字段 age: %v", err)
	}
}

func TestValidateArguments_TypeMismatch(t *testing.T) {
	schema := mustJSON(t, `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}`)
	err := ValidateArguments(schema, map[string]interface{}{"name": "alice", "age": "not-int"})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("期望 ErrInvalidArguments，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "age") {
		t.Fatalf("错误信息应指出字段 age 类型不匹配: %v", err)
	}
}

func TestValidateArguments_ExtraFieldsPass(t *testing.T) {
	// 额外字段不在 properties 中：放行（前向兼容）
	schema := mustJSON(t, `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)
	err := ValidateArguments(schema, map[string]interface{}{
		"name":    "alice",
		"unknown": 123,
	})
	if err != nil {
		t.Fatalf("额外字段应放行，实际 %v", err)
	}
}

func TestValidateArguments_AllTypesMatched(t *testing.T) {
	// 覆盖各类基本类型通过
	schema := mustJSON(t, `{"type":"object","properties":{
		"s":{"type":"string"},
		"i":{"type":"integer"},
		"n":{"type":"number"},
		"b":{"type":"boolean"},
		"o":{"type":"object"},
		"a":{"type":"array"}
	},"required":["s","i","n","b","o","a"]}`)
	args := map[string]interface{}{
		"s": "x",
		"i": float64(42),
		"n": float64(3.14),
		"b": true,
		"o": map[string]interface{}{"k": "v"},
		"a": []interface{}{1, 2},
	}
	if err := ValidateArguments(schema, args); err != nil {
		t.Fatalf("所有类型匹配应放行，实际 %v", err)
	}
}

func TestValidateArguments_IntegerAllowsFloatWithZeroFraction(t *testing.T) {
	// JSON 反序列化把整数也变成 float64(1.0)，应被识别为 integer
	schema := mustJSON(t, `{"type":"object","properties":{"i":{"type":"integer"}},"required":["i"]}`)
	if err := ValidateArguments(schema, map[string]interface{}{"i": float64(1.0)}); err != nil {
		t.Fatalf("float64(1.0) 应被识别为 integer，实际 %v", err)
	}
	// 3.14 不应被识别为 integer
	err := ValidateArguments(schema, map[string]interface{}{"i": float64(3.14)})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("3.14 不应通过 integer 校验，实际 %v", err)
	}
}

// ---- 版本校验 ----

func TestValidateVersion_NoClientVersionPasses(t *testing.T) {
	// 客户端未传 version：跳过校验
	tool := &model.MCPTool{Version: "1.0.0"}
	err := ValidateVersion(tool, map[string]interface{}{"name": "alice"})
	if err != nil {
		t.Fatalf("客户端未传 version 应放行，实际 %v", err)
	}
}

func TestValidateVersion_EmptyToolVersionPasses(t *testing.T) {
	// 工具未登记版本：跳过校验
	tool := &model.MCPTool{Version: ""}
	err := ValidateVersion(tool, map[string]interface{}{"version": "1.0.0"})
	if err != nil {
		t.Fatalf("工具未登记版本应放行，实际 %v", err)
	}
}

func TestValidateVersion_NilToolPasses(t *testing.T) {
	if err := ValidateVersion(nil, map[string]interface{}{"version": "1.0.0"}); err != nil {
		t.Fatalf("nil tool 应放行，实际 %v", err)
	}
}

func TestValidateVersion_NonStringVersionPasses(t *testing.T) {
	// version 非字符串：视为未指定，放行
	tool := &model.MCPTool{Version: "1.0.0"}
	err := ValidateVersion(tool, map[string]interface{}{"version": 1})
	if err != nil {
		t.Fatalf("非字符串 version 应放行，实际 %v", err)
	}
}

func TestValidateVersion_Mismatch(t *testing.T) {
	tool := &model.MCPTool{Version: "1.0.0"}
	err := ValidateVersion(tool, map[string]interface{}{"version": "2.0.0"})
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("期望 ErrVersionMismatch，实际 %v", err)
	}
}

func TestValidateVersion_Match(t *testing.T) {
	tool := &model.MCPTool{Version: "1.0.0"}
	err := ValidateVersion(tool, map[string]interface{}{"version": "1.0.0"})
	if err != nil {
		t.Fatalf("版本一致应放行，实际 %v", err)
	}
}
