-- 工具版本历史表
CREATE TABLE IF NOT EXISTS tool_versions (
    id            BIGSERIAL PRIMARY KEY,
    tool_id       BIGINT NOT NULL,
    version       VARCHAR(50) NOT NULL,
    input_schema  JSONB NOT NULL DEFAULT '{}',
    changelog     TEXT NOT NULL DEFAULT '',
    status        VARCHAR(20) NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'deprecated', 'retired')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tool_id) REFERENCES mcp_tools(id) ON DELETE CASCADE,
    UNIQUE (tool_id, version)
);

CREATE INDEX idx_tool_versions_tool_id ON tool_versions(tool_id);
