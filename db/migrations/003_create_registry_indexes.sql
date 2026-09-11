CREATE INDEX IF NOT EXISTS idx_mcp_servers_status
    ON mcp_servers (status);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_health_status
    ON mcp_servers (health_status);

CREATE INDEX IF NOT EXISTS idx_mcp_tools_server_id
    ON mcp_tools (server_id);

CREATE INDEX IF NOT EXISTS idx_mcp_tools_published
    ON mcp_tools (published);

CREATE INDEX IF NOT EXISTS idx_mcp_tools_health_status
    ON mcp_tools (health_status);

CREATE INDEX IF NOT EXISTS idx_mcp_tools_category
    ON mcp_tools (category);
