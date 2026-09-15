-- 初始化演示数据
-- 用户密码（bcrypt cost=10）：
--   admin/admin123、developer/dev123、agent/agent123

-- 用户：admin（密码 admin123）
INSERT INTO users (username, password_hash, role, status)
VALUES ('admin', '$2a$10$nn2H45ejGILTWakO/aVAc.r1A.LH539uIjsdOfKt8bGXbiKPB/Die', 'admin', 'active')
ON CONFLICT (username) DO NOTHING;

-- 用户：developer（密码 dev123）
INSERT INTO users (username, password_hash, role, status)
VALUES ('developer', '$2a$10$bixp1sMVt2DpJ7OkvLEi4.ZTD9eQ5av3h.3oMT2tAbYAJ.x/pdQZu', 'developer', 'active')
ON CONFLICT (username) DO NOTHING;

-- 用户：agent（密码 agent123）
INSERT INTO users (username, password_hash, role, status)
VALUES ('agent', '$2a$10$l2yX0emInbCaj4zj2XIUlOK/5ry4zonvgFh3ez.P6Z65Gex2lzwS.', 'agent', 'active')
ON CONFLICT (username) DO NOTHING;

-- 角色权限：admin 拥有所有工具（通配 *）
INSERT INTO role_tools (role, tool_name) VALUES ('admin', '*') ON CONFLICT DO NOTHING;
INSERT INTO role_tools (role, tool_name) VALUES ('developer', 'query_sales') ON CONFLICT DO NOTHING;
INSERT INTO role_tools (role, tool_name) VALUES ('developer', 'query_inventory') ON CONFLICT DO NOTHING;
INSERT INTO role_tools (role, tool_name) VALUES ('agent', 'query_sales') ON CONFLICT DO NOTHING;
-- agent 无 delete_customer 权限（用于演示敏感工具拒绝）

-- 注册示例 MCP Server（端点指向 compose 网络中的 db-server）
INSERT INTO mcp_servers (name, description, endpoint, version, status, health_status)
VALUES ('demo-db-server', '演示数据库 MCP Server', 'http://db-server:9004', '1.0.0', 'draft', 'unknown')
ON CONFLICT (name) DO NOTHING;

-- 注册示例工具
INSERT INTO mcp_tools (server_id, name, description, category, tags, input_schema, version, published, health_status)
SELECT id, 'query_sales', '查询脱敏销售数据', 'database', '{"db","report"}', '{"type":"object","properties":{"month":{"type":"string"}},"required":["month"]}', '1.0.0', false, 'unknown'
FROM mcp_servers WHERE name = 'demo-db-server'
ON CONFLICT (server_id, name) DO NOTHING;
