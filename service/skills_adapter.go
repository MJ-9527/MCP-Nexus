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

// SkillsService Skills → MCP 工具适配器（规范化转换，不含 Skills 运行时）
type SkillsService struct {
	tools    repository.ToolRepository
	servers  repository.ServerRepository
	endpoint string // Skill 执行入口默认指回网关（skills 转换后的调用统一走网关）
}

func NewSkillsService(tools repository.ToolRepository, servers repository.ServerRepository) *SkillsService {
	return &SkillsService{tools: tools, servers: servers}
}

// Import 批量把规范化 Skills 转换为 MCP 工具并注册。
// 一个导入请求对应一个虚拟 Server（endpoint 指向第一个 skill 的 endpoint），
// 工具名统一加 "skill_" 前缀避免与现有工具冲突。
func (s *SkillsService) Import(ctx context.Context, req *model.SkillImportRequest) (*model.SkillImportResult, error) {
	if len(req.Skills) == 0 {
		return nil, fmt.Errorf("skills 不能为空")
	}
	now := time.Now()
	// 幂等：同名 Server 已存在则复用，避免重复导入产生副本
	serverName := req.RegisterName
	if serverName == "" {
		serverName = "skills-imported"
	}
	endpoint := req.Skills[0].Endpoint
	// 先按名称查找，再按 endpoint 查找（endpoint 有 UNIQUE 约束）
	srv, err := s.servers.FindByName(ctx, serverName)
	if err != nil {
		srv, err = s.servers.FindByEndpoint(ctx, endpoint)
	}
	if err != nil {
		srv = &model.MCPServer{
			Name:        serverName,
			Description: "Skills 适配器导入（" + fmt.Sprintf("%d 个 skill）", len(req.Skills)),
			Endpoint:    endpoint,
			Version:     "1.0.0",
			Status:      "draft", HealthStatus: "unknown",
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.servers.Create(ctx, srv); err != nil {
			return nil, fmt.Errorf("注册 Skills Server 失败: %w", err)
		}
	}

	result := &model.SkillImportResult{ServerID: srv.ID}
	for _, sk := range req.Skills {
		name := "skill_" + strings.ToLower(strings.NewReplacer("-", "_", " ", "_").Replace(sk.Name))
		// 幂等：同名工具已存在则复用
		if existing, err := s.tools.FindByName(ctx, srv.ID, name); err == nil {
			result.Tools = append(result.Tools, *existing)
			continue
		}
		if sk.Parameters == nil {
			sk.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		// 规范化：确保是合法的 JSON Schema object
		if t, _ := sk.Parameters["type"].(string); t == "" {
			sk.Parameters["type"] = "object"
		}
		schemaJSON, err := json.Marshal(sk.Parameters)
		if err != nil {
			return nil, fmt.Errorf("skill %s 参数 Schema 非法: %w", sk.Name, err)
		}
		category := sk.Category
		if category == "" {
			category = "skills"
		}
		tool := &model.MCPTool{
			ServerID: srv.ID, Name: name,
			Description: "[Skill] " + sk.Description,
			Category:    category, Tags: sk.Tags,
			InputSchema: schemaJSON, Version: sk.Version,
			Published: false, HealthStatus: "unknown",
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.tools.Create(ctx, tool); err != nil {
			return nil, fmt.Errorf("注册 skill %s 失败: %w", sk.Name, err)
		}
		result.Tools = append(result.Tools, *tool)
	}

	// 生成接入配置片段
	snippet := map[string]any{
		"mcpServers": map[string]any{
			"skills": map[string]any{
				"url":       endpoint,
				"transport": "http",
				"note":      "Skill 经 Skills 适配器转换为 MCP 工具，调用走网关 /mcp/tools/{name}/call",
			},
		},
	}
	raw, _ := json.MarshalIndent(snippet, "", "  ")
	result.Snippet = raw
	return result, nil
}
