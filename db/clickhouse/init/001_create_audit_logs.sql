-- B14 审计分析存储（ClickHouse）。
-- 承接网关每次工具调用的审计记录，用于耗时 / 成功率 / 上游状态等分析型查询。
-- 字段与 model.AuditLog、PostgreSQL 的 audit_logs 表一一对齐。
--
-- 与 PostgreSQL 的差异：
--   - ClickHouse 无自增主键与外键约束，故不设 id / FK
--   - 排序键取 (created_at, request_id)，兼顾时间范围扫描与单请求定位
--   - params 只保存脱敏后的 sha256 摘要，原文永不落库
--
-- 建表脚本由官方镜像在 CLICKHOUSE_DB 指定的库中执行。
CREATE TABLE IF NOT EXISTS mcp_audit_logs
(
	 event_date              Date DEFAULT toDate(created_at),
    request_id              String,
    user_id                 Nullable(Int64),
    tool_id                 Nullable(Int64),
    server_id               Nullable(Int64),
    tool_name               String,
    caller_role             String,
    duration_ms             Int64,
    status                  LowCardinality(String),
    http_status             Int32,
    denied_reason           String,
    reject_reason           String,
    params_summary          String,
    params_sensitive_masked Bool,
    params_digest           String,
    cost_estimate           Float64,
    created_at              DateTime64(3, 'UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_date, created_at, ifNull(tool_id, 0), request_id)
TTL event_date + INTERVAL 180 DAY;
