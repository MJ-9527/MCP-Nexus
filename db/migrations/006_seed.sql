-- ============================================================================
-- seed.sql — 演示初始化数据
--
-- 依赖成员 A 的迁移脚本先建表：
--   001_create_mcp_servers.sql  -> mcp_servers
--   002_create_mcp_tools.sql    -> mcp_tools
--
-- 约定（需与成员 A 对齐，否则执行会失败）：
--   1. 脚本幂等，可重复执行（用 WHERE NOT EXISTS 判断，不依赖唯一约束）。
--   2. tags 为 TEXT[]，input_schema 为 JSONB（与成员 A 的迁移脚本一致）。
--   3. id / owner_id / server_id 为 BIGSERIAL/BIGINT（与 model.go 的 int64 一致）。
--   4. endpoint 使用 docker compose 网络内的服务名 demo-service:8081。
-- ============================================================================

-- 1. 演示 MCP Server：demo-db-server
INSERT INTO mcp_servers
    (name, description, endpoint, version, status, health_status, created_at, updated_at)
SELECT
    'demo-db-server',
    '演示数据库 MCP Server',
    'http://demo-service:8081',
    '1.0.0',
    'active',      -- status: 网关只允许 active 的 Server
    'unknown',     -- health_status: 由 POST /api/servers/:id/health-check 更新为 online
    now(),
    now()
WHERE NOT EXISTS (
    SELECT 1 FROM mcp_servers WHERE name = 'demo-db-server'
);

-- 2. query_sales（关联到 demo-db-server）
INSERT INTO mcp_tools
    (server_id, name, description, category, tags, input_schema, version, published, health_status, call_count, created_at, updated_at)
SELECT
    s.id,
    'query_sales',
    '查询脱敏销售数据',
    'database',
    ARRAY['sales','database']::text[],
    '{"type":"object","properties":{"month":{"type":"string"}}}'::jsonb,
    '1.0.0',
    true,          -- published: 网关只返回已发布的工具
    'online',
    0,
    now(),
    now()
FROM mcp_servers s
WHERE s.name = 'demo-db-server'
  AND NOT EXISTS (
      SELECT 1 FROM mcp_tools t
      WHERE t.server_id = s.id AND t.name = 'query_sales'
  );

-- 3. query_customer（数据库类示例工具，查询脱敏客户数据）
INSERT INTO mcp_tools
    (server_id, name, description, category, tags, input_schema, version, published, health_status, call_count, created_at, updated_at)
SELECT
    s.id,
    'query_customer',
    '查询脱敏客户数据',
    'database',
    ARRAY['customer','database']::text[],
    '{"type":"object","properties":{"region":{"type":"string"},"limit":{"type":"integer"}}}'::jsonb,
    '1.0.0',
    true,
    'online',
    0,
    now(),
    now()
FROM mcp_servers s
WHERE s.name = 'demo-db-server'
  AND NOT EXISTS (
      SELECT 1 FROM mcp_tools t
      WHERE t.server_id = s.id AND t.name = 'query_customer'
  );

-- 4. read_file（文件类示例工具，读取 base 目录内文件）
INSERT INTO mcp_tools
    (server_id, name, description, category, tags, input_schema, version, published, health_status, call_count, created_at, updated_at)
SELECT
    s.id,
    'read_file',
    '读取服务目录内的示例文件',
    'file',
    ARRAY['file','demo']::text[],
    '{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}'::jsonb,
    '1.0.0',
    true,
    'online',
    0,
    now(),
    now()
FROM mcp_servers s
WHERE s.name = 'demo-db-server'
  AND NOT EXISTS (
      SELECT 1 FROM mcp_tools t
      WHERE t.server_id = s.id AND t.name = 'read_file'
  );

-- 5. fetch_url（HTTP 类示例工具，请求白名单域名并脱敏响应）
INSERT INTO mcp_tools
    (server_id, name, description, category, tags, input_schema, version, published, health_status, call_count, created_at, updated_at)
SELECT
    s.id,
    'fetch_url',
    '请求白名单内的外部 URL 并返回脱敏响应',
    'http',
    ARRAY['http','fetch']::text[],
    '{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}'::jsonb,
    '1.0.0',
    true,
    'online',
    0,
    now(),
    now()
FROM mcp_servers s
WHERE s.name = 'demo-db-server'
  AND NOT EXISTS (
      SELECT 1 FROM mcp_tools t
      WHERE t.server_id = s.id AND t.name = 'fetch_url'
  );

-- 6. delete_customer（敏感工具，需 API Key，无权限时拒绝执行）
INSERT INTO mcp_tools
    (server_id, name, description, category, tags, input_schema, version, published, health_status, call_count, created_at, updated_at)
SELECT
    s.id,
    'delete_customer',
    '删除指定客户数据（敏感操作，需 API Key）',
    'database',
    ARRAY['sensitive','delete','customer']::text[],
    '{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"]}'::jsonb,
    '1.0.0',
    true,
    'online',
    0,
    now(),
    now()
FROM mcp_servers s
WHERE s.name = 'demo-db-server'
  AND NOT EXISTS (
      SELECT 1 FROM mcp_tools t
      WHERE t.server_id = s.id AND t.name = 'delete_customer'
  );
