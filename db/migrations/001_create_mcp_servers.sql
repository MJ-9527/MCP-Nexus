CREATE TABLE mcp_servers (
                             id BIGSERIAL PRIMARY KEY,
                             name VARCHAR(100) NOT NULL UNIQUE,
                             description TEXT NOT NULL DEFAULT '',
                             endpoint VARCHAR(500) NOT NULL UNIQUE,
                             version VARCHAR(50) NOT NULL,
                             owner_id BIGINT,
                             status VARCHAR(20) NOT NULL DEFAULT 'draft'
                                 CHECK (status IN ('draft', 'active', 'offline')),
                             health_status VARCHAR(20) NOT NULL DEFAULT 'unknown'
                                 CHECK (health_status IN ('unknown', 'online', 'degraded', 'offline')),
                             last_health_check_at TIMESTAMPTZ,
                             created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                             updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);