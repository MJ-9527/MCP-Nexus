CREATE TABLE tool_permissions (
    id BIGSERIAL PRIMARY KEY,
    tool_id BIGINT NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT REFERENCES roles(id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL DEFAULT 'call',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (num_nonnulls(user_id, role_id) = 1)
);

CREATE INDEX idx_tool_permissions_tool_id
    ON tool_permissions (tool_id);
CREATE INDEX idx_tool_permissions_user_id
    ON tool_permissions (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_tool_permissions_role_id
    ON tool_permissions (role_id) WHERE role_id IS NOT NULL;

CREATE UNIQUE INDEX uq_tool_permissions_user_action
    ON tool_permissions (tool_id, user_id, action) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX uq_tool_permissions_role_action
    ON tool_permissions (tool_id, role_id, action) WHERE role_id IS NOT NULL;
