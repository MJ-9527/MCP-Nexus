-- Core list queries: filter by dimension, then order/range by created_at.
CREATE INDEX IF NOT EXISTS idx_audit_logs_status_created ON audit_logs(status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tool_created ON audit_logs(tool_id, created_at DESC, id DESC) WHERE tool_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs(user_id, created_at DESC, id DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_server_created ON audit_logs(server_id, created_at DESC, id DESC) WHERE server_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_mcp_tools_published_popularity ON mcp_tools(published, call_count DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_tools_published_rating ON mcp_tools(published, average_rating DESC, rating_count DESC, id DESC);
DROP INDEX IF EXISTS idx_anomaly_alerts_status_triggered;
CREATE INDEX idx_anomaly_alerts_status_triggered ON anomaly_alerts(status, triggered_at DESC, id DESC);

-- PostgreSQL keeps recent business audit data online. Older rows are copied here
-- before deletion, allowing a longer compliance/archive window.
CREATE TABLE IF NOT EXISTS audit_logs_archive (LIKE audit_logs INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
ALTER TABLE audit_logs_archive ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE UNIQUE INDEX IF NOT EXISTS uq_audit_logs_archive_id ON audit_logs_archive(id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_archive_created ON audit_logs_archive(created_at DESC, id DESC);
