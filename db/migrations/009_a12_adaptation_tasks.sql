CREATE TABLE IF NOT EXISTS tool_adaptation_tasks (
    id BIGSERIAL PRIMARY KEY,
    tool_id BIGINT NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    task_type VARCHAR(32) NOT NULL CHECK (task_type IN ('openapi_import','skills_adaptation')),
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','success','failed')),
    source_url TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tool_adaptation_tasks_tool_created ON tool_adaptation_tasks(tool_id, created_at DESC);
