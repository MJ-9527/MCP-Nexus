CREATE TABLE IF NOT EXISTS anomaly_alerts (
    id BIGSERIAL PRIMARY KEY,
    alert_type VARCHAR(64) NOT NULL,
    severity VARCHAR(16) NOT NULL CHECK (severity IN ('low','medium','high','critical')),
    tool_id BIGINT REFERENCES mcp_tools(id) ON DELETE SET NULL,
    tool_name VARCHAR(100) NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged')),
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by BIGINT REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_anomaly_alerts_status_triggered ON anomaly_alerts(status, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_anomaly_alerts_tool_id ON anomaly_alerts(tool_id) WHERE tool_id IS NOT NULL;
