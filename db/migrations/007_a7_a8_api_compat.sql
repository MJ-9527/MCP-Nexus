ALTER TABLE mcp_tools
    ADD COLUMN IF NOT EXISTS is_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS sensitive_level VARCHAR(16);

ALTER TABLE mcp_tools DROP CONSTRAINT IF EXISTS mcp_tools_sensitive_level_check;
ALTER TABLE mcp_tools ADD CONSTRAINT mcp_tools_sensitive_level_check
    CHECK (sensitive_level IS NULL OR sensitive_level IN ('low', 'medium', 'high'));

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS server_id BIGINT REFERENCES mcp_servers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS tool_name VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS caller_role VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS http_status INTEGER NOT NULL DEFAULT 0 CHECK (http_status >= 0),
    ADD COLUMN IF NOT EXISTS params_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS params_sensitive_masked BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS cost_estimate NUMERIC(18, 6) NOT NULL DEFAULT 0 CHECK (cost_estimate >= 0),
    ADD COLUMN IF NOT EXISTS reject_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_audit_logs_server_id ON audit_logs (server_id) WHERE server_id IS NOT NULL;
