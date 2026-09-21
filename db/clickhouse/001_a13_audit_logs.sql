-- A13 analytical schema. PostgreSQL remains the source for business audit APIs.
CREATE TABLE IF NOT EXISTS mcp_audit_logs
(
    event_date Date DEFAULT toDate(created_at),
    created_at DateTime64(3, 'UTC'),
    request_id String,
    user_id Nullable(Int64),
    tool_id Nullable(Int64),
    server_id Nullable(Int64),
    tool_name LowCardinality(String),
    caller_role LowCardinality(String),
    status LowCardinality(String),
    http_status UInt16,
    duration_ms UInt64,
    denied_reason String,
    reject_reason String,
    params_digest FixedString(64),
    params_sensitive_masked UInt8,
    cost_estimate Float64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_date, created_at, ifNull(tool_id, 0), request_id)
TTL event_date + INTERVAL 180 DAY;
