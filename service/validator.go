package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"MCP-Nexus/model"
)

// 参数与版本校验（B12）。
//
// 设计目标：在调用转发前对客户端入参进行可预测的前置校验，校验失败立即返回 400 类
// 错误，避免无意义的下游请求与资源占用。校验范围限定在「最小可用的 JSON Schema 子集」：
//   - type=object（兼容 input_schema 缺省情况，自动视作 object）
//   - required：必填字段必须存在于 arguments
//   - properties：对 arguments 中出现的字段按 properties[name].type 校验类型
//   - 未在 properties 中声明的额外字段放行（前向兼容，避免老网关拒绝新客户端）
//
// 不实现的 JSON Schema 高级特性：pattern/format/enum/oneOf/anyOf/allof/min/max 等。
// 若后续确有需求，再引入第三方库替换本实现，避免一次性引入复杂依赖。
//
// 版本校验：arguments.version（字符串）存在且 tool.Version 非空时进行严格匹配；
// 客户端不传 version 视为不指定，跳过校验以兼容现有调用方。

// inputSchema 表示 input_schema 的最小结构。缺省字段视为允许。
type inputSchema struct {
	Type       string                     `json:"type"`       // 期望 "object"；空表示不限制
	Properties map[string]json.RawMessage `json:"properties"` // 各字段子 schema
	Required   []string                   `json:"required"`   // 必填字段名
}

// ValidateArguments 对客户端入参按工具 input_schema 进行校验（B12）。
//
// 行为约定：
//   - schema 为空 / 解析失败 → 放行（无法可依，不阻塞调用）
//   - schema.type 非 object 且非空 → 放行（非 object 的 schema 不在本校验器范围内）
//   - 缺少必填字段 → ErrInvalidArguments（列出所有缺失字段，便于客户端一次性修正）
//   - 字段类型不匹配 → ErrInvalidArguments（列出首个不匹配字段）
//   - 额外字段不在 properties 中 → 放行（前向兼容）
func ValidateArguments(schema json.RawMessage, args map[string]interface{}) error {
	if len(schema) == 0 || string(schema) == "null" {
		return nil
	}
	var s inputSchema
	if err := json.Unmarshal(schema, &s); err != nil {
		// schema 自身不合法：放行，避免把脏数据从导入阶段带到调用阶段
		return nil
	}
	if s.Type != "" && s.Type != "object" {
		// 仅校验 object 类型；其他类型（array/string 等）不在该校验器范围
		return nil
	}

	// 1. 必填字段检查
	missing := make([]string, 0)
	for _, name := range s.Required {
		if _, ok := args[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing required fields: %s", ErrInvalidArguments, strings.Join(missing, ", "))
	}

	// 2. 字段类型检查（仅对 properties 中声明且 args 中出现的字段）
	for name, raw := range s.Properties {
		val, ok := args[name]
		if !ok {
			continue // 未传的字段不校验类型（除非在 required 中，上面已处理）
		}
		if err := validatePropertyValue(name, raw, val); err != nil {
			return err
		}
	}
	return nil
}

// validatePropertyValue 校验单个字段值与子 schema 声明的类型一致。
func validatePropertyValue(name string, raw json.RawMessage, val interface{}) error {
	var sub struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &sub); err != nil || sub.Type == "" {
		// 子 schema 不合法或缺失 type：不校验，避免误伤
		return nil
	}
	if !valueMatchesType(val, sub.Type) {
		return fmt.Errorf("%w: field %q expected type %s", ErrInvalidArguments, name, sub.Type)
	}
	return nil
}

// valueMatchesType 判断 Go 值（来自 JSON 反序列化）是否匹配 JSON Schema 类型。
// 支持的类型：string/number/integer/boolean/object/array/null。
func valueMatchesType(val interface{}, typ string) bool {
	switch typ {
	case "string":
		_, ok := val.(string)
		return ok
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "number":
		// JSON 反序列化为 float64
		_, ok := val.(float64)
		return ok
	case "integer":
		f, ok := val.(float64)
		if !ok {
			return false
		}
		// 整数意味着无小数部分（兼容 1.0 表示整数 1）
		return f == float64(int64(f))
	case "object":
		_, ok := val.(map[string]interface{})
		return ok
	case "array":
		_, ok := val.([]interface{})
		return ok
	case "null":
		return val == nil
	default:
		// 未知类型不校验（兼容 type 缺省或扩展类型）
		return true
	}
}

// ValidateVersion 校验客户端指定的工具版本与当前发布版本是否一致（B12）。
//
// 行为约定：
//   - 客户端未在 arguments.version 中指定版本 → 跳过校验（兼容现有调用方）
//   - tool.Version 为空 → 跳过校验（工具未登记版本，无法对齐）
//   - 二者皆有且不相等 → ErrVersionMismatch
func ValidateVersion(tool *model.MCPTool, args map[string]interface{}) error {
	if tool == nil {
		return nil
	}
	if tool.Version == "" {
		return nil
	}
	raw, ok := args["version"]
	if !ok {
		return nil
	}
	ver, ok := raw.(string)
	if !ok {
		// version 字段非字符串：视为未指定版本，不阻塞调用
		return nil
	}
	if ver != tool.Version {
		return fmt.Errorf("%w: client requested %s, tool published %s", ErrVersionMismatch, ver, tool.Version)
	}
	return nil
}
