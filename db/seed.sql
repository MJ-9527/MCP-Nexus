-- Seed data for MCP-Nexus demo environment
-- Run after all migrations have been applied (cmd/db-migrate -seed db/seed.sql 或手工执行)。
-- 幂等：全部使用 WHERE NOT EXISTS / ON CONFLICT，可重复执行。
--
-- 角色命名遵循《does/变量命名与合并规范.md》：
--   platform_admin / tool_developer / agent_caller
--
-- 演示账号（密码均为 password123；下方为 bcrypt cost=10 哈希，禁止写明文）：
--   admin      platform_admin
--   dev01      tool_developer
--   agent01    agent_caller
--   restricted agent_caller（无任何工具授权，用于验证 403）

-- 1. Roles
INSERT INTO roles (name, description) VALUES
    ('platform_admin', '平台管理员，拥有全部权限'),
    ('tool_developer', '工具开发者，可注册和维护工具'),
    ('agent_caller',   'Agent 调用者，仅可调用已授权工具')
ON CONFLICT (name) DO NOTHING;

-- 2. Users (password for all: "password123", bcrypt hash)
INSERT INTO users (username, password_hash, status)
SELECT 'admin', '$2a$10$SUc114j.T/VDrNybR75iSenWIYwa3oSJzGRl/iU.Cl6I3Bx2Elob.', 'active'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'admin');
INSERT INTO users (username, password_hash, status)
SELECT 'dev01', '$2a$10$SUc114j.T/VDrNybR75iSenWIYwa3oSJzGRl/iU.Cl6I3Bx2Elob.', 'active'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'dev01');
INSERT INTO users (username, password_hash, status)
SELECT 'agent01', '$2a$10$SUc114j.T/VDrNybR75iSenWIYwa3oSJzGRl/iU.Cl6I3Bx2Elob.', 'active'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'agent01');
INSERT INTO users (username, password_hash, status)
SELECT 'restricted', '$2a$10$SUc114j.T/VDrNybR75iSenWIYwa3oSJzGRl/iU.Cl6I3Bx2Elob.', 'active'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'restricted');

-- 3. User-role assignments
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON r.name = 'platform_admin'
WHERE u.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id AND ur.role_id = r.id);
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON r.name = 'tool_developer'
WHERE u.username = 'dev01'
  AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id AND ur.role_id = r.id);
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON r.name = 'agent_caller'
WHERE u.username IN ('agent01', 'restricted')
  AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id AND ur.role_id = r.id);

-- 4. MCP Servers（端口与 deploy/docker-compose.yml 中 demo-service 的宿主映射一致）
INSERT INTO mcp_servers (name, description, endpoint, version, owner_id, status, health_status)
SELECT 'demo-db-server', '演示 MCP Server — 销售/客户查询与文件、HTTP、敏感操作示例',
       'http://demo-service:8081', '1.0.0',
       (SELECT id FROM users WHERE username = 'dev01'), 'active', 'unknown'
WHERE NOT EXISTS (SELECT 1 FROM mcp_servers WHERE name = 'demo-db-server');

-- 已存在的旧记录（如早期 seed 指向 9001）一并修正
UPDATE mcp_servers
SET endpoint = 'http://demo-service:8081', status = 'active', updated_at = NOW()
WHERE name = 'demo-db-server' AND endpoint <> 'http://demo-service:8081';

-- 5. MCP Tools（与 examples/demo-service 暴露的工具一一对应）
INSERT INTO mcp_tools (server_id, name, description, category, tags, input_schema, version, published, health_status)
SELECT s.id, v.name, v.description, v.category, v.tags, v.schema::jsonb, '1.0.0', true, 'online'
FROM mcp_servers s
CROSS JOIN (VALUES
    ('query_sales',     '查询脱敏销售数据',           'database', ARRAY['sales','report'],
     '{"type":"object","properties":{"month":{"type":"string","description":"月份 YYYY-MM"},"region":{"type":"string","description":"区域"}},"required":["month"]}'),
    ('query_customer',  '查询脱敏客户数据',           'database', ARRAY['customer','database'],
     '{"type":"object","properties":{"region":{"type":"string"},"limit":{"type":"integer"}}}'),
    ('list_products',   '列出所有产品（脱敏）',        'database', ARRAY['products','catalog'],
     '{"type":"object","properties":{"category":{"type":"string"}}}'),
    ('read_file',       '读取服务目录内的示例文件',    'file',     ARRAY['file','demo'],
     '{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}'),
    ('fetch_url',       '请求白名单内 URL 并脱敏响应', 'http',     ARRAY['http','fetch'],
     '{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}'),
    ('delete_customer', '删除客户记录（敏感操作，需 API Key）', 'database', ARRAY['customer','sensitive'],
     '{"type":"object","properties":{"customer_id":{"type":"integer"}},"required":["customer_id"]}')
) AS v(name, description, category, tags, schema)
WHERE s.name = 'demo-db-server'
  AND NOT EXISTS (SELECT 1 FROM mcp_tools t WHERE t.server_id = s.id AND t.name = v.name);

-- 6. Tool permissions（角色维度：网关 RBAC 经 roles.name 关联）
-- platform_admin：全部工具 view+call
INSERT INTO tool_permissions (tool_id, role_id, action)
SELECT t.id, r.id, a.action
FROM mcp_tools t
JOIN mcp_servers s ON s.id = t.server_id AND s.name = 'demo-db-server'
JOIN roles r ON r.name = 'platform_admin'
CROSS JOIN (VALUES ('view'), ('call')) AS a(action)
WHERE NOT EXISTS (
    SELECT 1 FROM tool_permissions p
    WHERE p.tool_id = t.id AND p.role_id = r.id AND p.action = a.action
);

-- agent_caller：非敏感工具 view+call；delete_customer 不授权（用于验证 403）
INSERT INTO tool_permissions (tool_id, role_id, action)
SELECT t.id, r.id, a.action
FROM mcp_tools t
JOIN mcp_servers s ON s.id = t.server_id AND s.name = 'demo-db-server'
JOIN roles r ON r.name = 'agent_caller'
CROSS JOIN (VALUES ('view'), ('call')) AS a(action)
WHERE t.name IN ('query_sales', 'query_customer', 'list_products', 'read_file', 'fetch_url')
  AND NOT EXISTS (
    SELECT 1 FROM tool_permissions p
    WHERE p.tool_id = t.id AND p.role_id = r.id AND p.action = a.action
);

-- 7. 用户直授（agent01 额外可调用 query_sales；与角色授权并存，验证授权并集）
INSERT INTO tool_permissions (tool_id, user_id, action)
SELECT t.id, u.id, 'call'
FROM mcp_tools t
JOIN mcp_servers s ON s.id = t.server_id AND s.name = 'demo-db-server'
JOIN users u ON u.username = 'agent01'
WHERE t.name = 'query_sales'
  AND NOT EXISTS (
    SELECT 1 FROM tool_permissions p
    WHERE p.tool_id = t.id AND p.user_id = u.id AND p.action = 'call'
);
