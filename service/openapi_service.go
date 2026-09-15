package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// OpenAPIService OpenAPI 导入编排：解析 → 校验 → 生成骨架 → 可选注册落库
type OpenAPIService struct {
	importer *OpenAPIImporter
	tools    repository.ToolRepository
	servers  repository.ServerRepository
}

func NewOpenAPIService(tools repository.ToolRepository, servers repository.ServerRepository) *OpenAPIService {
	return &OpenAPIService{importer: NewOpenAPIImporter(), tools: tools, servers: servers}
}

// Import 执行导入
func (s *OpenAPIService) Import(ctx context.Context, req *model.OpenAPIImportRequest) (*model.OpenAPIImportResult, error) {
	ps, err := s.importer.Parse(req.Spec)
	if err != nil {
		return nil, err
	}
	if req.BaseURL != "" {
		ps.BaseURL = strings.TrimRight(req.BaseURL, "/")
	}

	files, envTemplate := s.importer.GenerateSkeleton(ps)

	result := &model.OpenAPIImportResult{
		InfoTitle:    ps.Title,
		BaseURL:      ps.BaseURL,
		AuthType:     ps.AuthType,
		EnvTemplate:  envTemplate,
		StartupGuide: startupGuide(),
	}
	for _, f := range files {
		result.Files = append(result.Files, model.GeneratedFile{Path: f.Path, Content: f.Content})
	}

	// 注册：把每个操作创建为 mcp_tools 记录（默认 draft，由管理员审核后发布）
	// 幂等：同名 Server / 同名工具已存在时直接复用，支持重复导入
	if req.AutoRegister {
		serverName := req.RegisterName
		if serverName == "" {
			serverName = ps.Title
			if serverName == "" {
				serverName = "openapi-imported"
			}
		}
		now := time.Now()
		srv, err := s.servers.FindByName(ctx, serverName)
		if err != nil {
			// endpoint 有 UNIQUE 约束，同名不存在时再按 endpoint 查找
			srv, err = s.servers.FindByEndpoint(ctx, ps.BaseURL)
		}
		if err != nil {
			srv = &model.MCPServer{
				Name: serverName, Description: "OpenAPI 自动包装：" + ps.Title,
				Endpoint: ps.BaseURL, Version: "1.0.0",
				Status: "draft", HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
			}
			if err := s.servers.Create(ctx, srv); err != nil {
				return nil, fmt.Errorf("注册 Server 失败: %w", err)
			}
		}
		result.ServerID = &srv.ID
		for _, op := range ps.Ops {
			if existing, err := s.tools.FindByName(ctx, srv.ID, op.ToolName); err == nil {
				result.Tools = append(result.Tools, *existing)
				continue
			}
			schemaJSON, _ := json.Marshal(op.InputSchema)
			tool := &model.MCPTool{
				ServerID: srv.ID, Name: op.ToolName, Description: op.Description,
				Category: "openapi", InputSchema: schemaJSON, Version: "1.0.0",
				Published: false, HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now,
			}
			if err := s.tools.Create(ctx, tool); err != nil {
				return nil, fmt.Errorf("注册工具 %s 失败: %w", op.ToolName, err)
			}
			result.Tools = append(result.Tools, *tool)
		}
	}
	return result, nil
}

func startupGuide() string {
	return strings.Join([]string{
		"1. 解压生成文件到独立目录（main.go / Dockerfile / .env.example）",
		"2. cp .env.example .env 并填写 UPSTREAM_BASE_URL 与认证变量",
		"3. 本地运行：go mod init mcp-server && go mod tidy && go run .",
		"4. 容器运行：docker build -t my-mcp-server . && docker run -p 9100:9100 --env-file .env my-mcp-server",
		"5. 验证：GET http://localhost:9100/health",
		"6. 回到网关：POST /api/servers 注册（endpoint=http://host:9100），POST /api/tools 逐个注册生成的工具定义，审核后发布",
	}, "\n")
}
