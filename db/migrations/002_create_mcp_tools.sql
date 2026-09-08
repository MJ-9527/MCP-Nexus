CREATE TABLE mcp_tools (
    id BIGSERIAL PRIMARY KEY ,
    server_id BIGINT NOT NULL ,
    name VARCHAR(100) NOT NULL ,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(100) NOT NULL ,
    tags TEXT[] NOT NULL DEFAULT '{}' ,
    input_schema JSONB NOT NULL DEFAULT '{}',
    version VARCHAR(50) NOT NULL ,
    published BOOLEAN NOT NULL  DEFAULT FALSE,
    health_status VARCHAR(20) NOT NULL DEFAULT 'unknown'
                         CHECK ( health_status IN('unknown', 'online', 'degraded', 'offline')),
    call_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL  DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL  DEFAULT NOW(),

    FOREIGN KEY (server_id)
                         REFERENCES mcp_servers(id)
                         ON DELETE CASCADE ,

    UNIQUE (server_id,name)
);