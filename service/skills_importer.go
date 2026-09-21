package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// SkillsImporter 编排 Skills 适配任务工具列表的导入：
//  1. 校验目标 Server 存在
//  2. 逐个校验并落地 SkillsToolDef：在 mcp_tools 表创建/复用工具（幂等：同 Server 下同名工具复用）
//  3. 在 mcp_tool_skills_specs 表 UPSERT 调用元数据（endpoint/method/path/params）
//
// 与 OpenAPIImporter 的差异：输入是结构化的工具定义（适配任务已经准备好 name/description/schema/endpoint），
// 不需要解析规范文档；导入器只做落地与翻译元数据保存。
//
// 导入后工具默认未发布（Published=false），管理员审核后显式发布，避免误曝光未校验的上游 Skills 工具。
// 客户端可通过请求体 published=true 要求导入即发布（适用于已受信的内部 Skills 服务）。
type SkillsImporter struct {
	serverRepo repository.ServerRepository
	toolRepo   repository.ToolRepository
	specRepo   repository.SkillsSpecRepository
}

func NewSkillsImporter(
	serverRepo repository.ServerRepository,
	toolRepo repository.ToolRepository,
	specRepo repository.SkillsSpecRepository,
) *SkillsImporter {
	return &SkillsImporter{
		serverRepo: serverRepo,
		toolRepo:   toolRepo,
		specRepo:   specRepo,
	}
}

// Import 落地 Skills 工具列表 + 翻译元数据。
// 幂等：同 Server 下同名工具复用（更新 spec 而非报冲突），支持重复导入修正 spec。
func (i *SkillsImporter) Import(ctx context.Context, serverID int64, req model.ImportSkillsRequest) (*model.ImportSkillsResult, error) {
	if i == nil || i.serverRepo == nil || i.toolRepo == nil || i.specRepo == nil {
		return nil, ErrInvalidServer
	}
	if serverID <= 0 {
		return nil, ErrInvalidServer
	}
	server, err := i.serverRepo.FindByID(ctx, serverID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServerNotFound
	} else if err != nil {
		return nil, err
	}
	// 基本校验：工具名非空、不重复（请求体内）
	if err := validateSkillsTools(req.Tools); err != nil {
		return nil, err
	}
	result := &model.ImportSkillsResult{
		ServerID: serverID,
		Imported: make([]model.ImportedSkillsEntry, 0, len(req.Tools)),
	}
	for _, def := range req.Tools {
		entry, err := i.upsertTool(ctx, server, def, req.Published)
		if err != nil {
			return nil, fmt.Errorf("import skills tool %s: %w", def.Name, err)
		}
		result.Imported = append(result.Imported, entry)
	}
	result.Total = len(result.Imported)
	return result, nil
}

// validateSkillsTools 校验工具定义合法性：名称非空、InputSchema 合法 JSON、请求体内不重名。
func validateSkillsTools(tools []model.SkillsToolDef) error {
	seen := make(map[string]bool, len(tools))
	for _, def := range tools {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			return fmt.Errorf("%w: tool name is empty", ErrInvalidSkills)
		}
		if len(def.InputSchema) == 0 || !json.Valid(def.InputSchema) {
			return fmt.Errorf("%w: tool %s input_schema is invalid json", ErrInvalidSkills, name)
		}
		if seen[name] {
			return fmt.Errorf("%w: duplicate tool name %s in request", ErrInvalidSkills, name)
		}
		seen[name] = true
	}
	return nil
}

// upsertTool 对单个 SkillsToolDef 幂等落地：tool 已存在则更新描述/schema/版本 + 刷新 spec，
// 不存在则创建。Category 固定为 "skills"，区分原生 MCP 与 OpenAPI 工具。
func (i *SkillsImporter) upsertTool(ctx context.Context, server *model.MCPServer, def model.SkillsToolDef, published bool) (model.ImportedSkillsEntry, error) {
	name := strings.TrimSpace(def.Name)
	existing, err := i.toolRepo.FindByName(ctx, server.ID, name)
	if err == nil && existing != nil {
		// 复用：更新描述/版本/入参 schema + 刷新 spec
		if strings.TrimSpace(def.Description) != "" {
			existing.Description = def.Description
		}
		if len(def.InputSchema) > 0 {
			existing.InputSchema = def.InputSchema
		}
		existing.Version = fallbackString(def.Version, fallbackString(server.Version, existing.Version))
		existing.Category = "skills" // 强制归一，防止历史脏数据
		if err := i.saveSpec(ctx, existing.ID, def); err != nil {
			return model.ImportedSkillsEntry{}, err
		}
		return model.ImportedSkillsEntry{
			ToolID: existing.ID,
			Name:   existing.Name,
			Action: "updated",
		}, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return model.ImportedSkillsEntry{}, err
	}
	// 新建工具（约束：导入必须幂等，但同名工具不存在时才 Create）
	tool := &model.MCPTool{
		ServerID:     server.ID,
		Name:         name,
		Description:  strings.TrimSpace(def.Description),
		Category:     "skills",
		Tags:         []string{},
		InputSchema:  def.InputSchema,
		Version:      fallbackString(def.Version, server.Version),
		Published:    published,
		HealthStatus: "unknown",
	}
	if err := i.toolRepo.Create(ctx, tool); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			// 并发或种子数据导致冲突：回退到更新路径
			return i.updateExistingByName(ctx, server, def)
		}
		return model.ImportedSkillsEntry{}, err
	}
	if err := i.saveSpec(ctx, tool.ID, def); err != nil {
		return model.ImportedSkillsEntry{}, err
	}
	return model.ImportedSkillsEntry{
		ToolID: tool.ID,
		Name:   tool.Name,
		Action: "created",
	}, nil
}

func (i *SkillsImporter) updateExistingByName(ctx context.Context, server *model.MCPServer, def model.SkillsToolDef) (model.ImportedSkillsEntry, error) {
	existing, err := i.toolRepo.FindByName(ctx, server.ID, strings.TrimSpace(def.Name))
	if err != nil {
		return model.ImportedSkillsEntry{}, err
	}
	if err := i.saveSpec(ctx, existing.ID, def); err != nil {
		return model.ImportedSkillsEntry{}, err
	}
	return model.ImportedSkillsEntry{
		ToolID: existing.ID,
		Name:   existing.Name,
		Action: "updated",
	}, nil
}

// saveSpec 序列化 SkillsParams 并 UPSERT 调用元数据。
// 默认值：Method 空则 POST；PathTemplate 空则 /tools/{name}/call（原生 MCP 兼容路径）。
func (i *SkillsImporter) saveSpec(ctx context.Context, toolID int64, def model.SkillsToolDef) error {
	method := strings.TrimSpace(def.Method)
	if method == "" {
		method = "POST"
	}
	pathTemplate := strings.TrimSpace(def.PathTemplate)
	if pathTemplate == "" {
		// 默认走原生 MCP 路径，保证 Skills 工具可以接入到标准 MCP Server
		pathTemplate = "/tools/" + strings.TrimSpace(def.Name) + "/call"
	}
	paramsBytes, err := json.Marshal(def.Params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	return i.specRepo.Save(ctx, &model.SkillsSpec{
		ToolID:       toolID,
		Endpoint:     strings.TrimSpace(def.Endpoint),
		Method:       strings.ToUpper(method),
		PathTemplate: pathTemplate,
		Params:       paramsBytes,
	})
}
