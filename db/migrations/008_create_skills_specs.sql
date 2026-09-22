-- B11 Skills 适配器对接：存储从 Skills 适配任务导入的工具调用元数据。
-- 每条记录关联一个 mcp_tools 行（tool_id 唯一），记录 endpoint/method/path_template/params，
-- 供 ProxyService 在调用时将 MCP 请求翻译为实际 HTTP 请求发送到上游 Skills 服务。
-- 与 mcp_tool_openapi_specs 的差异：endpoint 字段允许每个 Skills 工具有独立的调用地址。
CREATE TABLE mcp_tool_skills_specs (
    id             BIGSERIAL PRIMARY KEY,
    tool_id        BIGINT NOT NULL UNIQUE,
    endpoint       VARCHAR(500) NOT NULL DEFAULT '',  -- 空=复用 server.Endpoint
    method         VARCHAR(10)  NOT NULL DEFAULT 'POST',
    path_template  VARCHAR(500) NOT NULL DEFAULT '',  -- 空=用 /tools/{name}/call 原生路径
    params         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE
);

CREATE INDEX idx_skills_specs_tool_id ON mcp_tool_skills_specs(tool_id);
