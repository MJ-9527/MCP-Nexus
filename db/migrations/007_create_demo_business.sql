-- ============================================================================
-- 007_create_demo_business.sql — 演示业务数据（成员 C / C5 数据库类示例工具）
--
-- 供 demo-service 的 query_customer 工具查询，演示「数据库类示例工具」。
-- 数据全部为脱敏样例（姓名打码、手机号/邮箱脱敏），不含真实 PII。
-- ============================================================================

CREATE TABLE IF NOT EXISTS demo_customers (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    phone      TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    region     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 幂等插入脱敏演示数据
INSERT INTO demo_customers (name, phone, email, region)
SELECT v.name, v.phone, v.email, v.region
FROM (VALUES
    ('张**', '138****5678', 'zhang***@example.com', '华东'),
    ('李**', '139****1234', 'li***@example.com',     '华北'),
    ('王**', '137****9999', 'wang***@example.com',   '华南'),
    ('赵**', '136****0001', 'zhao***@example.com',   '华东')
) AS v(name, phone, email, region)
WHERE NOT EXISTS (SELECT 1 FROM demo_customers);
