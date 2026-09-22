package service

import (
	"encoding/json"
	"strings"
	"testing"

	"MCP-Nexus/model"
)

// petStoreSpec 最小可用 OpenAPI 3.0 规范：覆盖 GET 带 path 参数 / GET 带 query /
// POST 带 JSON body / DELETE 四类典型 operation。
const petStoreSpec = `{
  "openapi": "3.0.0",
  "info": {"title": "Pet Store", "version": "1.0.0"},
  "paths": {
    "/pets": {
      "get": {
        "operationId": "listPets",
        "summary": "List all pets",
        "parameters": [
          {"name": "limit", "in": "query", "schema": {"type": "integer"}}
        ]
      },
      "post": {
        "operationId": "createPet",
        "summary": "Create a pet",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "object", "properties": {"name": {"type": "string"}}}
            }
          }
        }
      }
    },
    "/pets/{petId}": {
      "get": {
        "operationId": "showPetById",
        "summary": "Info for a specific pet",
        "parameters": [
          {"name": "petId", "in": "path", "required": true, "schema": {"type": "string"}}
        ]
      },
      "delete": {
        "operationId": "deletePet",
        "summary": "Delete a pet",
        "parameters": [
          {"name": "petId", "in": "path", "required": true, "schema": {"type": "string"}}
        ]
      }
    }
  }
}`

func TestOpenAPIParser_Parse(t *testing.T) {
	p := NewOpenAPIParser()
	ops, err := p.Parse(json.RawMessage(petStoreSpec))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 4 {
		t.Fatalf("期望 4 个 operation，实际 %d", len(ops))
	}
	byName := map[string]OpenAPIOperation{}
	for _, op := range ops {
		byName[op.OperationID] = op
	}
	cases := []struct {
		name        string
		method      string
		path        string
		pathParams  []string
		queryParams []string
		bodyKey     string
	}{
		{"listPets", "GET", "/pets", nil, []string{"limit"}, ""},
		{"createPet", "POST", "/pets", nil, nil, "body"},
		{"showPetById", "GET", "/pets/{petId}", []string{"petId"}, nil, ""},
		{"deletePet", "DELETE", "/pets/{petId}", []string{"petId"}, nil, ""},
	}
	for _, c := range cases {
		op, ok := byName[c.name]
		if !ok {
			t.Errorf("缺少 operation %q", c.name)
			continue
		}
		if op.Method != c.method {
			t.Errorf("%s: method 期望 %s 实际 %s", c.name, c.method, op.Method)
		}
		if op.Path != c.path {
			t.Errorf("%s: path 期望 %s 实际 %s", c.name, c.path, op.Path)
		}
		if !sliceEqual(op.Params.Path, c.pathParams) {
			t.Errorf("%s: path params 期望 %v 实际 %v", c.name, c.pathParams, op.Params.Path)
		}
		if !sliceEqual(op.Params.Query, c.queryParams) {
			t.Errorf("%s: query params 期望 %v 实际 %v", c.name, c.queryParams, op.Params.Query)
		}
		if op.Params.Body != c.bodyKey {
			t.Errorf("%s: body 期望 %q 实际 %q", c.name, c.bodyKey, op.Params.Body)
		}
		if len(op.InputSchema) == 0 {
			t.Errorf("%s: input_schema 不应为空", c.name)
		}
	}
}

func TestOpenAPIParser_InvalidSpec(t *testing.T) {
	p := NewOpenAPIParser()
	cases := []struct {
		name string
		spec string
	}{
		{"空 JSON", ""},
		{"非法 JSON", "{not json"},
		{"缺少版本", `{"paths":{}}`},
		{"缺少 paths", `{"openapi":"3.0.0"}`},
		{"空 paths", `{"openapi":"3.0.0","paths":{}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := p.Parse(json.RawMessage(c.spec))
			if err == nil {
				t.Fatalf("期望错误，实际 nil")
			}
			if !strings.Contains(err.Error(), "openapi spec") && !strings.Contains(err.Error(), "paths") && !strings.Contains(err.Error(), "version") {
				t.Logf("错误信息: %v", err)
			}
		})
	}
}

func TestOpenAPIParser_DefaultOperationID(t *testing.T) {
	// operationId 缺失时用 method+path 生成默认名
	spec := `{"openapi":"3.0.0","paths":{"/users/{id}":{"get":{"summary":"get user","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}]}}}}`
	p := NewOpenAPIParser()
	ops, err := p.Parse(json.RawMessage(spec))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 {
		t.Fatalf("期望 1 个 operation，实际 %d", len(ops))
	}
	if ops[0].OperationID != "get_users_id" {
		t.Fatalf("默认 operationId 期望 get_users_id，实际 %s", ops[0].OperationID)
	}
}

// swagger2Spec 最小 Swagger 2.0 规范：用 in=body 表示请求体。
const swagger2Spec = `{
  "swagger": "2.0",
  "info": {"title": "Old API", "version": "1.0"},
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "parameters": [
          {"name": "q", "in": "query", "type": "string"}
        ]
      },
      "post": {
        "operationId": "createItem",
        "parameters": [
          {"name": "body", "in": "body", "schema": {"type": "object"}}
        ]
      }
    }
  }
}`

func TestOpenAPIParser_Swagger2(t *testing.T) {
	p := NewOpenAPIParser()
	ops, err := p.Parse(json.RawMessage(swagger2Spec))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("期望 2 个 operation，实际 %d", len(ops))
	}
	for _, op := range ops {
		if op.OperationID == "createItem" && op.Params.Body != "body" {
			t.Errorf("Swagger 2.0 in=body 应识别为 body 参数，实际 %q", op.Params.Body)
		}
		if op.OperationID == "listItems" && len(op.Params.Query) != 1 {
			t.Errorf("listItems query 参数期望 1 个，实际 %d", len(op.Params.Query))
		}
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 防止 model 未引用被 vet 误报
var _ = model.OpenAPISpec{}
