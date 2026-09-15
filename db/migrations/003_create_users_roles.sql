-- 用户表（复用 model.User 结构）
CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    username        VARCHAR(64)  NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(32)  NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'disabled')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 角色-工具权限映射表（RBAC 核心）
CREATE TABLE IF NOT EXISTS role_tools (
    role         VARCHAR(32)  NOT NULL,
    tool_name    VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role, tool_name)
);

CREATE INDEX idx_role_tools_role ON role_tools(role);
