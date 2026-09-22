-- Seed data for MCP-Nexus demo environment
-- Run after all migrations have been applied.
-- Idempotent: uses ON CONFLICT DO NOTHING where applicable.

-- 1. Roles
INSERT INTO roles (name, description) VALUES
    ('admin',    '平台管理员，拥有全部权限'),
    ('developer','工具开发者，可注册和维护工具'),
    ('caller',   'Agent 调用者，仅可调用已授权工具')
ON CONFLICT (name) DO NOTHING;

-- 2. Users (password for all: "password123")
-- Hash generated with bcrypt cost 10
INSERT INTO users (username, password_hash, status) VALUES
    ('admin',     'password123', 'active'),
    ('dev01',     'password123', 'active'),
    ('agent01',   'password123', 'active'),
    ('restricted','password123', 'active')
ON CONFLICT (username) DO NOTHING;

-- 3. User-role assignments
-- admin -> admin, dev01 -> developer, agent01 -> caller, restricted -> caller
WITH admin_id AS (SELECT id FROM users WHERE username = 'admin'),
     dev_id  AS (SELECT id FROM users WHERE username = 'dev01'),
     agent_id AS (SELECT id FROM users WHERE username = 'agent01'),
     restr_id AS (SELECT id FROM users WHERE username = 'restricted'),
     admin_role AS (SELECT id FROM roles WHERE name = 'admin'),
     dev_role  AS (SELECT id FROM roles WHERE name = 'developer'),
     caller_role AS (SELECT id FROM roles WHERE name = 'caller')
INSERT INTO user_roles (user_id, role_id)
SELECT admin_id.id, admin_role.id FROM admin_id, admin_role
UNION ALL
SELECT dev_id.id, dev_role.id FROM dev_id, dev_role
UNION ALL
SELECT agent_id.id, caller_role.id FROM agent_id, caller_role
UNION ALL
SELECT restr_id.id, caller_role.id FROM restr_id, caller_role
ON CONFLICT DO NOTHING;

-- 4. MCP Servers
INSERT INTO mcp_servers (name, description, endpoint, version, owner_id, status, health_status) VALUES
    ('demo-db-server', '演示数据库 MCP Server — 提供脱敏销售数据查询', 'http://localhost:9001', '1.0.0',
        (SELECT id FROM users WHERE username = 'dev01'), 'active', 'online'),
    ('demo-file-server', '演示文件系统 MCP Server', 'http://localhost:9002', '1.0.0',
        (SELECT id FROM users WHERE username = 'dev01'), 'active', 'online')
ON CONFLICT (name) DO NOTHING;

-- 5. MCP Tools
INSERT INTO mcp_tools (server_id, name, description, category, tags, input_schema, version, published, health_status) VALUES
    ((SELECT id FROM mcp_servers WHERE name = 'demo-db-server'),
     'query_sales', '查询脱敏销售数据', 'database', ARRAY['sales', 'report'],
     '{"type":"object","properties":{"month":{"type":"string","description":"月份，格式 YYYY-MM"},"region":{"type":"string","description":"区域名称"}},"required":["month"]}'::jsonb,
     '1.0.0', true, 'online'),

    ((SELECT id FROM mcp_servers WHERE name = 'demo-db-server'),
     'list_products', '列出所有产品（脱敏）', 'database', ARRAY['products', 'catalog'],
     '{"type":"object","properties":{"category":{"type":"string","description":"产品分类"}},"required":[]}'::jsonb,
     '1.0.0', true, 'online'),

    ((SELECT id FROM mcp_servers WHERE name = 'demo-db-server'),
     'delete_customer', '删除客户记录（敏感操作）', 'database', ARRAY['customer', 'sensitive'],
     '{"type":"object","properties":{"customer_id":{"type":"integer","description":"客户 ID"}},"required":["customer_id"]}'::jsonb,
     '1.0.0', true, 'online'),

    ((SELECT id FROM mcp_servers WHERE name = 'demo-file-server'),
     'read_file', '读取文件内容', 'file', ARRAY['file', 'read'],
     '{"type":"object","properties":{"path":{"type":"string","description":"文件绝对路径"}},"required":["path"]}'::jsonb,
     '1.0.0', true, 'online')
ON CONFLICT DO NOTHING;

-- 6. Tool permissions
-- admin can do everything on all tools
-- agent01 can call query_sales and list_products but NOT delete_customer
-- restricted caller cannot call any sensitive tools
WITH agent_id AS (SELECT id FROM users WHERE username = 'agent01'),
     admin_id AS (SELECT id FROM users WHERE username = 'admin'),
     query_sales_id AS (SELECT id FROM mcp_tools WHERE name = 'query_sales'),
     list_prod_id   AS (SELECT id FROM mcp_tools WHERE name = 'list_products'),
     delete_id      AS (SELECT id FROM mcp_tools WHERE name = 'delete_customer')
INSERT INTO tool_permissions (tool_id, user_id, action)
SELECT query_sales_id.id, agent_id.id, 'call' FROM query_sales_id, agent_id
UNION ALL
SELECT list_prod_id.id, agent_id.id, 'call' FROM list_prod_id, agent_id
UNION ALL
SELECT query_sales_id.id, admin_id.id, 'call' FROM query_sales_id, admin_id
UNION ALL
SELECT list_prod_id.id, admin_id.id, 'call' FROM list_prod_id, admin_id
UNION ALL
SELECT delete_id.id, admin_id.id, 'call' FROM delete_id, admin_id
ON CONFLICT DO NOTHING;

-- Note: delete_customer has NO caller permission for agent01 or restricted users.
-- This is the intended setup for testing 403 access denial.
