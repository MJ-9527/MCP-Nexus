-- ============================================================================
-- seed.sql — 演示初始化数据（成员 C）
--
-- 依赖成员 A 的迁移脚本先建表：
--   001_create_mcp_servers.sql  -> mcp_servers
--   002_create_mcp_tools.sql    -> mcp_tools
--
-- 约定（需与成员 A 对齐，否则执行会失败）：
--   1. 脚本幂等，可重复执行（用 WHERE NOT EXISTS 判断，不依赖唯一约束）。
--   2. tags / input_schema 假定为 JSONB；若成员 A 用 TEXT[] 需同步改。
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

-- 2. 演示工具：query_sales（关联到上面的 demo-db-server）
INSERT INTO mcp_tools
    (server_id, name, description, category, tags, input_schema, version, published, health_status, call_count, created_at, updated_at)
SELECT
    s.id,
    'query_sales',
    '查询脱敏销售数据',
    'database',
    '["sales", "database"]'::jsonb,
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
