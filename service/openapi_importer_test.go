package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

const sampleSpecJSON = `{
  "openapi": "3.0.0",
  "info": {"title": "Pet Store", "version": "1.0.0"},
  "servers": [{"url": "https://petstore.example.com/api"}],
  "paths": {
    "/pets/{petId}": {
      "get": {
        "operationId": "getPetById",
        "summary": "按 ID 查询宠物",
        "parameters": [
          {"name": "petId", "in": "path", "required": true, "schema": {"type": "integer"}},
          {"$ref": "#/components/parameters/LimitHeader"}
        ]
      }
    },
    "/pets": {
      "post": {
        "operationId": "createPet",
        "summary": "创建宠物",
        "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/NewPet"}}}}
      }
    }
  },
  "components": {
    "parameters": {"LimitHeader": {"name": "X-Limit", "in": "header", "required": false, "schema": {"type": "integer"}}},
    "schemas": {"NewPet": {"type": "object", "properties": {"name": {"type": "string"}, "tag": {"type": "string"}}, "required": ["name"]}}
  }
}`

const sampleSpecYAML = `
openapi: 3.0.0
info:
  title: Demo API
  version: 1.0.0
servers:
  - url: http://demo.example.com
paths:
  /greet:
    get:
      operationId: greet
      summary: 问候
      parameters:
        - name: name
          in: query
          required: true
          schema:
            type: string
`

func TestOpenAPIImporter_ParseJSON(t *testing.T) {
	im := NewOpenAPIImporter()
	ps, err := im.Parse(sampleSpecJSON)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	if ps.Title != "Pet Store" || ps.BaseURL != "https://petstore.example.com/api" {
		t.Fatalf("基础信息错误: %+v", ps)
	}
	if len(ps.Ops) != 2 {
		t.Fatalf("应解析出 2 个操作（delete 应忽略并报错前）: got %d", len(ps.Ops))
	}
	// getPetById：path + header 参数合并
	var getOp *parsedOp
	var postOp *parsedOp
	for i := range ps.Ops {
		switch ps.Ops[i].ToolName {
		case "getPetById":
			getOp = &ps.Ops[i]
		case "createPet":
			postOp = &ps.Ops[i]
		}
	}
	if getOp == nil || postOp == nil {
		t.Fatalf("缺少预期操作: %+v", ps.Ops)
	}
	if len(getOp.PathParams) != 1 || getOp.PathParams[0] != "petId" {
		t.Fatalf("path 参数解析错误: %v", getOp.PathParams)
	}
	if len(getOp.HeaderParams) != 1 || getOp.HeaderParams[0] != "X-Limit" {
		t.Fatalf("$ref header 参数解析错误: %v", getOp.HeaderParams)
	}
	if !postOp.HasBody {
		t.Fatal("createPet 应有 body")
	}
	props := postOp.InputSchema["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Fatalf("body object 应展开合并 properties: %v", props)
	}
	req, _ := postOp.InputSchema["required"].([]string)
	if len(req) == 0 || req[0] != "name" {
		t.Fatalf("body required 未合并: %v", req)
	}
}

func TestOpenAPIImporter_ParseYAML(t *testing.T) {
	im := NewOpenAPIImporter()
	ps, err := im.Parse(sampleSpecYAML)
	if err != nil {
		t.Fatalf("Parse YAML 失败: %v", err)
	}
	if len(ps.Ops) != 1 || ps.Ops[0].ToolName != "greet" {
		t.Fatalf("YAML 解析错误: %+v", ps.Ops)
	}
}

func TestOpenAPIImporter_RejectUnsupported(t *testing.T) {
	im := NewOpenAPIImporter()

	// PUT 不支持
	putSpec := strings.Replace(sampleSpecJSON, `"/pets": {`, `"/pets": {"put": {"operationId": "x"},`, 1)
	if _, err := im.Parse(putSpec); err == nil || !strings.Contains(err.Error(), "PUT") {
		t.Fatalf("PUT 应明确报错: %v", err)
	}

	// oauth2 安全方案不支持
	oauthSpec := `{"openapi":"3.0.0","info":{"title":"x","version":"1"},"servers":[{"url":"http://a.com"}],
	  "security":[{"oauth2Scheme":[]}],
	  "components":{"securitySchemes":{"oauth2Scheme":{"type":"oauth2","flows":{}}}},
	  "paths":{"/a":{"get":{"operationId":"a"}}}}`
	if _, err := im.Parse(oauthSpec); err == nil || !strings.Contains(err.Error(), "oauth2") {
		t.Fatalf("oauth2 应明确报错: %v", err)
	}

	// 非 JSON 请求体不支持
	xmlSpec := strings.Replace(sampleSpecJSON, `"application/json": {"schema": {"$ref": "#/components/schemas/NewPet"}}`, `"application/xml": {"schema": {"type": "object"}}`, 1)
	if _, err := im.Parse(xmlSpec); err == nil || !strings.Contains(err.Error(), "application/xml") {
		t.Fatalf("application/xml 应明确报错: %v", err)
	}

	// 外部 $ref 不支持
	extSpec := `{"openapi":"3.0.0","info":{"title":"x","version":"1"},"servers":[{"url":"http://a.com"}],
	  "paths":{"/a":{"get":{"operationId":"a","parameters":[{"$ref":"http://other/doc#/p"}]}}}}`
	if _, err := im.Parse(extSpec); err == nil || !strings.Contains(err.Error(), "外部") {
		t.Fatalf("外部 $ref 应明确报错: %v", err)
	}
}

func TestOpenAPIImporter_GenerateSkeleton(t *testing.T) {
	im := NewOpenAPIImporter()
	ps, err := im.Parse(sampleSpecJSON)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	files, env := im.GenerateSkeleton(ps)
	if len(files) != 3 {
		t.Fatalf("应生成 3 个文件: %v", files)
	}
	var mainGo string
	for _, f := range files {
		if f.Path == "main.go" {
			mainGo = f.Content
		}
	}
	for _, want := range []string{"package main", "getPetById", "createPet", "UPSTREAM_BASE_URL", "net/url"} {
		if !strings.Contains(mainGo, want) {
			t.Fatalf("骨架缺少 %q", want)
		}
	}
	if !strings.Contains(env, "UPSTREAM_BASE_URL=https://petstore.example.com/api") {
		t.Fatalf("env 模板错误: %s", env)
	}
}

func TestMarketService_ListAndReview(t *testing.T) {
	tools := repository.NewMemoryToolRepository()
	servers := repository.NewMemoryServerRepository()
	reviews := repository.NewMemoryReviewRepository()
	svc := NewMarketService(tools, servers, reviews, nil)

	_ = servers.Create(context.Background(), &model.MCPServer{ID: 1, Name: "srv", Status: "active"})
	tools.TestPutTool(&model.MCPTool{ID: 1, ServerID: 1, Name: "t1", Description: "search tool", Published: true, Category: "db"})
	tools.TestPutTool(&model.MCPTool{ID: 2, ServerID: 1, Name: "t2", Published: false})
	tools.TestPutTool(&model.MCPTool{ID: 3, ServerID: 1, Name: "t3", Published: true, HealthStatus: "offline"})

	// 列表：只显示 published 且非 offline
	list, err := svc.List(context.Background(), "", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "t1" {
		t.Fatalf("列表过滤错误: %+v", list.Items)
	}

	// 关键词搜索
	list, _ = svc.List(context.Background(), "", "search", 1, 20)
	if len(list.Items) != 1 {
		t.Fatalf("关键词搜索失败: %+v", list.Items)
	}

	// 评分 upsert
	if _, err := svc.UpsertReview(context.Background(), 1, 100, &model.UpsertReviewRequest{Rating: 5, Comment: "好用"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertReview(context.Background(), 1, 100, &model.UpsertReviewRequest{Rating: 4, Comment: "不错"}); err != nil {
		t.Fatal(err)
	}
	stats, _ := reviews.StatsByTool(context.Background(), 1)
	if stats.Count != 1 || stats.AvgRating != 4 {
		t.Fatalf("upsert 评分统计错误: %+v", stats)
	}

	// 配置片段
	snippet, err := svc.ConfigSnippet(context.Background(), 1, "http://gw:8080")
	if err != nil || !strings.Contains(string(snippet.Snippet), "/mcp/tools/t1/call") {
		t.Fatalf("配置片段错误: %v %s", err, snippet.Snippet)
	}

	// 下线工具片段应拒绝
	if _, err := svc.ConfigSnippet(context.Background(), 3, ""); !errors.Is(err, ErrToolOffline) {
		t.Fatalf("下线工具应返回 TOOL_OFFLINE: %v", err)
	}
}
