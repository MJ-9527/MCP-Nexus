CREATE TABLE tool_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE mcp_tools
    ADD COLUMN category_id BIGINT REFERENCES tool_categories(id) ON DELETE SET NULL,
    ADD COLUMN average_rating NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (average_rating BETWEEN 0 AND 5),
    ADD COLUMN rating_count BIGINT NOT NULL DEFAULT 0 CHECK (rating_count >= 0);

CREATE TABLE tool_versions (
    id BIGSERIAL PRIMARY KEY,
    tool_id BIGINT NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    input_schema JSONB NOT NULL DEFAULT '{}',
    changelog TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'deprecated')),
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tool_id, version)
);

CREATE UNIQUE INDEX uq_tool_versions_one_current
    ON tool_versions (tool_id) WHERE is_current = TRUE;

CREATE TABLE tool_ratings (
    id BIGSERIAL PRIMARY KEY,
    tool_id BIGINT NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tool_id, user_id)
);

CREATE INDEX idx_mcp_tools_category_id ON mcp_tools (category_id);
CREATE INDEX idx_mcp_tools_average_rating ON mcp_tools (average_rating DESC);
CREATE INDEX idx_mcp_tools_call_count ON mcp_tools (call_count DESC);
CREATE INDEX idx_mcp_tools_tags_gin ON mcp_tools USING GIN (tags);
CREATE INDEX idx_mcp_tools_market_search ON mcp_tools USING GIN (
    to_tsvector('simple', name || ' ' || description)
);
CREATE INDEX idx_tool_versions_tool_id ON tool_versions (tool_id);
CREATE INDEX idx_tool_ratings_tool_created ON tool_ratings (tool_id, created_at DESC);
