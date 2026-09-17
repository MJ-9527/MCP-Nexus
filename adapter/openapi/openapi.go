// Package openapi 实现 MVP 限定的 OpenAPI 3.x → MCP 生成器（成员 C）。
//
// 支持子集：GET/POST、path/query/header/body 参数、API Key 与 Bearer Token、
// 基础响应 Schema。其余特性在 Validate 阶段返回明确错误，绝不静默生成错误代码。
package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// OpenAPI 3.x 受限子集的数据结构
// ---------------------------------------------------------------------------

type Spec struct {
	OpenAPI    string                `yaml:"openapi" json:"openapi"`
	Info       Info                  `yaml:"info" json:"info"`
	Servers    []Server              `yaml:"servers" json:"servers"`
	Paths      map[string]*PathItem  `yaml:"paths" json:"paths"`
	Components *Components           `yaml:"components" json:"components"`
	Security   []SecurityRequirement `yaml:"security" json:"security"`
}

type Info struct {
	Title       string `yaml:"title" json:"title"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
}

type Server struct {
	URL         string `yaml:"url" json:"url"`
	Description string `yaml:"description" json:"description"`
}

type PathItem struct {
	Get    *Operation `yaml:"get" json:"get"`
	Post   *Operation `yaml:"post" json:"post"`
	Put    *Operation `yaml:"put" json:"put"`
	Delete *Operation `yaml:"delete" json:"delete"`
	Patch  *Operation `yaml:"patch" json:"patch"`
}

type Operation struct {
	OperationID string                `yaml:"operationId" json:"operationId"`
	Summary     string                `yaml:"summary" json:"summary"`
	Description string                `yaml:"description" json:"description"`
	Parameters  []Parameter           `yaml:"parameters" json:"parameters"`
	RequestBody *RequestBody          `yaml:"requestBody" json:"requestBody"`
	Responses   map[string]*Response  `yaml:"responses" json:"responses"`
	Security    []SecurityRequirement `yaml:"security" json:"security"`
}

type Parameter struct {
	Name        string  `yaml:"name" json:"name"`
	In          string  `yaml:"in" json:"in"`
	Required    bool    `yaml:"required" json:"required"`
	Description string  `yaml:"description" json:"description"`
	Schema      *Schema `yaml:"schema" json:"schema"`
}

type RequestBody struct {
	Required bool                  `yaml:"required" json:"required"`
	Content  map[string]*MediaType `yaml:"content" json:"content"`
}

type MediaType struct {
	Schema *Schema `yaml:"schema" json:"schema"`
}

type Response struct {
	Description string                `yaml:"description" json:"description"`
	Content     map[string]*MediaType `yaml:"content" json:"content"`
}

type Schema struct {
	Type        string             `yaml:"type" json:"type"`
	Format      string             `yaml:"format" json:"format"`
	Description string             `yaml:"description" json:"description"`
	Properties  map[string]*Schema `yaml:"properties" json:"properties"`
	Items       *Schema            `yaml:"items" json:"items"`
	Required    []string           `yaml:"required" json:"required"`
	Default     any                `yaml:"default" json:"default"`
	Ref         string             `yaml:"$ref" json:"$ref"`
	Minimum     *float64           `yaml:"minimum" json:"minimum"`
	Maximum     *float64           `yaml:"maximum" json:"maximum"`
	Enum        []any              `yaml:"enum" json:"enum"`
}

type Components struct {
	SecuritySchemes map[string]*SecurityScheme `yaml:"securitySchemes" json:"securitySchemes"`
	Schemas         map[string]*Schema         `yaml:"schemas" json:"schemas"`
}

type SecurityScheme struct {
	Type         string `yaml:"type" json:"type"`
	In           string `yaml:"in" json:"in"`
	Name         string `yaml:"name" json:"name"`
	Scheme       string `yaml:"scheme" json:"scheme"`
	BearerFormat string `yaml:"bearerFormat" json:"bearerFormat"`
}

// SecurityRequirement 表示一个鉴权要求：security scheme 名 → 所需 scopes（MVP 忽略 scopes）。
type SecurityRequirement map[string][]string

// ---------------------------------------------------------------------------
// 解析
// ---------------------------------------------------------------------------

// Parse 解析 OpenAPI 3.x 文档（JSON 或 YAML，按首个非空字符自动识别）。
func Parse(data []byte) (*Spec, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty spec")
	}
	var spec Spec
	if data[0] == '{' {
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
	}
	return &spec, nil
}

// ---------------------------------------------------------------------------
// 校验（不支持的特性必须返回明确错误）
// ---------------------------------------------------------------------------

// Validate 校验 Spec 是否落在支持的子集内，返回聚合后的所有错误。
func Validate(spec *Spec) error {
	if spec == nil {
		return errors.New("nil spec")
	}
	if spec.OpenAPI == "" {
		return errors.New("missing openapi version")
	}
	if !strings.HasPrefix(spec.OpenAPI, "3.") {
		return fmt.Errorf("unsupported OpenAPI version %q (only 3.x supported)", spec.OpenAPI)
	}

	var errs []error
	for path, item := range spec.Paths {
		if item == nil {
			errs = append(errs, fmt.Errorf("path %q: empty path item", path))
			continue
		}
		for _, m := range []struct {
			method string
			op     *Operation
		}{
			{"get", item.Get}, {"post", item.Post}, {"put", item.Put},
			{"delete", item.Delete}, {"patch", item.Patch},
		} {
			if m.op == nil {
				continue
			}
			if m.method != "get" && m.method != "post" {
				errs = append(errs, fmt.Errorf("%s %s: unsupported HTTP method %q (only GET/POST supported)", strings.ToUpper(m.method), path, strings.ToUpper(m.method)))
				continue
			}
			errs = append(errs, validateOperation(spec, path, m.method, m.op)...)
		}
	}

	if spec.Components != nil {
		for name, ss := range spec.Components.SecuritySchemes {
			errs = append(errs, validateSecurityScheme(name, ss)...)
		}
	}
	return errors.Join(errs...)
}

func validateOperation(spec *Spec, path, method string, op *Operation) []error {
	var errs []error
	prefix := strings.ToUpper(method) + " " + path

	if op.OperationID == "" {
		errs = append(errs, fmt.Errorf("%s: missing operationId (tool name cannot be derived)", prefix))
	}

	for _, p := range op.Parameters {
		switch p.In {
		case "path", "query", "header":
		case "":
			errs = append(errs, fmt.Errorf("%s: parameter %q missing 'in'", prefix, p.Name))
		default:
			errs = append(errs, fmt.Errorf("%s: unsupported parameter location %q for %q (only path/query/header supported)", prefix, p.In, p.Name))
		}
		if p.Schema != nil {
			errs = append(errs, validateSchema(spec, p.Schema, prefix+" param "+p.Name)...)
		}
	}

	if op.RequestBody != nil && len(op.RequestBody.Content) > 0 {
		mt, ok := op.RequestBody.Content["application/json"]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: unsupported request body content type (only application/json supported)", prefix))
		} else if mt.Schema != nil {
			errs = append(errs, validateSchema(spec, mt.Schema, prefix+" body")...)
		}
	}
	return errs
}

func validateSchema(spec *Spec, s *Schema, where string) []error {
	var errs []error
	s = resolveSchema(spec, s)
	if s == nil {
		return errs
	}
	if s.Type != "" {
		switch s.Type {
		case "string", "integer", "number", "boolean", "array", "object":
		default:
			errs = append(errs, fmt.Errorf("%s: unsupported schema type %q", where, s.Type))
		}
	}
	for k, v := range s.Properties {
		errs = append(errs, validateSchema(spec, v, where+"."+k)...)
	}
	if s.Items != nil {
		errs = append(errs, validateSchema(spec, s.Items, where+"[]")...)
	}
	return errs
}

func validateSecurityScheme(name string, ss *SecurityScheme) []error {
	var errs []error
	switch ss.Type {
	case "apiKey":
		if ss.In != "header" {
			errs = append(errs, fmt.Errorf("security scheme %q: unsupported apiKey 'in' %q (only header supported)", name, ss.In))
		}
		if ss.Name == "" {
			errs = append(errs, fmt.Errorf("security scheme %q: missing apiKey name", name))
		}
	case "http":
		if ss.Scheme != "bearer" {
			errs = append(errs, fmt.Errorf("security scheme %q: unsupported http scheme %q (only bearer supported)", name, ss.Scheme))
		}
	case "":
		errs = append(errs, fmt.Errorf("security scheme %q: missing type", name))
	default:
		errs = append(errs, fmt.Errorf("security scheme %q: unsupported type %q (only apiKey/http supported)", name, ss.Type))
	}
	return errs
}

// resolveSchema 就地解析本地 $ref（#/components/schemas/X）。
func resolveSchema(spec *Spec, s *Schema) *Schema {
	for i := 0; s != nil && s.Ref != "" && i < 10; i++ {
		name := strings.TrimPrefix(s.Ref, "#/components/schemas/")
		if spec.Components == nil {
			break
		}
		next, ok := spec.Components.Schemas[name]
		if !ok {
			break
		}
		s = next
	}
	return s
}
