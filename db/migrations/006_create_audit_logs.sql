CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    tool_id BIGINT REFERENCES mcp_tools(id) ON DELETE SET NULL,
    duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    status VARCHAR(32) NOT NULL,
    denied_reason TEXT NOT NULL DEFAULT '',
    params_digest CHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_request_id ON audit_logs (request_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_logs_tool_id ON audit_logs (tool_id) WHERE tool_id IS NOT NULL;
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at DESC);
