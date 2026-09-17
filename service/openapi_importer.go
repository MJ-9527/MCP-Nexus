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

// OpenAPIImporter 编排 OpenAPI 规范导入：
//  1. 校验目标 Server 存在
//  2. 调用 OpenAPIParser 将 spec 解析为 operations
//  3. 为每个 operation 在 mcp_tools 表创建/复用工具（幂等：同 Server 下同名工具复用）
//  4. 在 mcp_tool_openapi_specs 表 UPSERT 翻译元数据（method/path/params）
//
// 导入后工具默认未发布（Published=false），管理员审核后显式发布，避免误曝光未校验的上游接口。
// 客户端可通过请求体 published=true 要求导入即发布（适用于已受信的内部 API）。
type OpenAPIImporter struct {
	serverRepo repository.ServerRepository
	toolRepo   repository.ToolRepository
	specRepo   repository.OpenAPISpecRepository
	parser     *OpenAPIParser
}

func NewOpenAPIImporter(
	serverRepo repository.ServerRepository,
	toolRepo repository.ToolRepository,
	specRepo repository.OpenAPISpecRepository,
) *OpenAPIImporter {
	return &OpenAPIImporter{
		serverRepo: serverRepo,
		toolRepo:   toolRepo,
		specRepo:   specRepo,
		parser:     NewOpenAPIParser(),
	}
}

// Import 解析 spec 并为每个 operation 落地 tool + spec。
// 幂等：同 Server 下同名工具复用（更新 spec 而非报冲突），支持重复导入修正 spec。
func (i *OpenAPIImporter) Import(ctx context.Context, serverID int64, req model.ImportOpenAPIRequest) (*model.ImportOpenAPIResult, error) {
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
	operations, err := i.parser.Parse(req.Spec)
	if err != nil {
		return nil, err
	}
	result := &model.ImportOpenAPIResult{
		ServerID: serverID,
		Imported: make([]model.ImportedToolEntry, 0, len(operations)),
	}
	for _, op := range operations {
		entry, err := i.upsertOperation(ctx, server, op, req.Published)
		if err != nil {
			return nil, fmt.Errorf("import operation %s %s: %w", op.Method, op.Path, err)
		}
		result.Imported = append(result.Imported, entry)
	}
	result.Total = len(result.Imported)
	return result, nil
}

// upsertOperation 对单个 operation 幂等落地：tool 已存在则更新 spec，不存在则创建。
func (i *OpenAPIImporter) upsertOperation(ctx context.Context, server *model.MCPServer, op OpenAPIOperation, published bool) (model.ImportedToolEntry, error) {
	existing, err := i.toolRepo.FindByName(ctx, server.ID, op.OperationID)
	if err == nil && existing != nil {
		// 复用：更新描述/版本/入参 schema + 刷新 spec
		existing.Description = fallbackString(op.Summary, existing.Description)
		if len(op.InputSchema) > 0 {
			existing.InputSchema = op.InputSchema
		}
		existing.Version = fallbackString(server.Version, existing.Version)
		if err := i.saveSpec(ctx, existing.ID, op); err != nil {
			return model.ImportedToolEntry{}, err
		}
		return model.ImportedToolEntry{
			ToolID: existing.ID,
			Name:   existing.Name,
			Action: "updated",
		}, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return model.ImportedToolEntry{}, err
	}
	// 新建工具（约束：导入必须幂等，但同名工具不存在时才 Create）
	tool := &model.MCPTool{
		ServerID:     server.ID,
		Name:         op.OperationID,
		Description:  strings.TrimSpace(op.Summary),
		Category:     "openapi",
		Tags:         []string{},
		InputSchema:  op.InputSchema,
		Version:      server.Version,
		Published:    published,
		HealthStatus: "unknown",
	}
	if err := i.toolRepo.Create(ctx, tool); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			// 并发或种子数据导致冲突：回退到更新路径
			return i.updateExistingByName(ctx, server, op)
		}
		return model.ImportedToolEntry{}, err
	}
	if err := i.saveSpec(ctx, tool.ID, op); err != nil {
		return model.ImportedToolEntry{}, err
	}
	return model.ImportedToolEntry{
		ToolID: tool.ID,
		Name:   tool.Name,
		Action: "created",
	}, nil
}

func (i *OpenAPIImporter) updateExistingByName(ctx context.Context, server *model.MCPServer, op OpenAPIOperation) (model.ImportedToolEntry, error) {
	existing, err := i.toolRepo.FindByName(ctx, server.ID, op.OperationID)
	if err != nil {
		return model.ImportedToolEntry{}, err
	}
	if err := i.saveSpec(ctx, existing.ID, op); err != nil {
		return model.ImportedToolEntry{}, err
	}
	return model.ImportedToolEntry{
		ToolID: existing.ID,
		Name:   existing.Name,
		Action: "updated",
	}, nil
}

// saveSpec 序列化 OpenAPIParams 并 UPSERT 翻译元数据。
func (i *OpenAPIImporter) saveSpec(ctx context.Context, toolID int64, op OpenAPIOperation) error {
	paramsBytes, err := json.Marshal(op.Params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	return i.specRepo.Save(ctx, &model.OpenAPISpec{
		ToolID:       toolID,
		Method:       op.Method,
		PathTemplate: op.Path,
		Params:       paramsBytes,
	})
}

// fallbackString 返回第一个非空字符串，用于"导入值缺失时保留原值"。
func fallbackString(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
