-- 评分评论表：每个用户对每个工具只有一条评论，可重复提交更新
CREATE TABLE reviews (
                        id BIGSERIAL PRIMARY KEY,
                        tool_id BIGINT NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
                        user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                        rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
                        comment TEXT NOT NULL DEFAULT '',
                        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                        UNIQUE (tool_id, user_id)
);

CREATE INDEX idx_reviews_tool ON reviews(tool_id);
