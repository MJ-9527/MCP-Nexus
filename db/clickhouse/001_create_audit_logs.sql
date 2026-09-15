-- ClickHouse DDL：审计日志表
-- 由 clickhouse 容器 initdb 执行（注意 ClickHouse 语法与 PostgreSQL 不同）
CREATE TABLE IF NOT EXISTS audit_logs (
    request_id      String,
    caller_id       Int64,
    role            String,
    tool_name       String,
    server_id       Int64,
    args_summary    String,
    called_at       DateTime,
    latency_ms      Int32,
    success         UInt8,
    http_status     Int32,
    reject_reason   String,
    cost_estimate   Float32
) ENGINE = MergeTree()
ORDER BY (called_at, tool_name)
PARTITION BY toYYYYMM(called_at);
