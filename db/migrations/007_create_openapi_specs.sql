-- B10 OpenAPI 包装器对接：存储从 OpenAPI/Swagger 规范解析出的工具翻译元数据。
-- 每条记录关联一个 mcp_tools 行（tool_id 唯一），记录 method/path_template/params 分类，
-- 供 ProxyService 在调用时将 MCP 请求翻译为实际 HTTP 请求发送到上游 API。
CREATE TABLE mcp_tool_openapi_specs (
    id             BIGSERIAL PRIMARY KEY,
    tool_id        BIGINT NOT NULL UNIQUE,
    method         VARCHAR(10)  NOT NULL,
    path_template  VARCHAR(500) NOT NULL,
    params         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE
);

CREATE INDEX idx_openapi_specs_tool_id ON mcp_tool_openapi_specs(tool_id);
