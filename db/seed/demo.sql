CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO roles(name,description) VALUES
('platform_admin','平台管理员'),('tool_developer','工具开发者'),('agent_caller','Agent 调用者')
ON CONFLICT(name) DO UPDATE SET description=EXCLUDED.description;

INSERT INTO users(username,password_hash,status) VALUES
('demo_admin',crypt('DemoAdmin123!',gen_salt('bf',10)),'active'),
('demo_developer',crypt('DemoDeveloper123!',gen_salt('bf',10)),'active'),
('demo_agent',crypt('DemoAgent123!',gen_salt('bf',10)),'active')
ON CONFLICT(username) DO UPDATE SET status='active';

INSERT INTO user_roles(user_id,role_id)
SELECT u.id,r.id FROM (VALUES('demo_admin','platform_admin'),('demo_developer','tool_developer'),('demo_agent','agent_caller')) v(username,role_name)
JOIN users u ON u.username=v.username JOIN roles r ON r.name=v.role_name ON CONFLICT DO NOTHING;

INSERT INTO mcp_servers(name,description,endpoint,version,owner_id,status,health_status)
SELECT 'Demo MCP Server','A16 可联调演示服务','http://localhost:9090/mcp','1.0.0',u.id,'active','online' FROM users u WHERE u.username='demo_developer'
ON CONFLICT(name) DO UPDATE SET description=EXCLUDED.description,status='active',health_status='online';

INSERT INTO tool_categories(name,slug,description) VALUES('Database','database','数据库工具')
ON CONFLICT(slug) DO UPDATE SET description=EXCLUDED.description;

INSERT INTO mcp_tools(server_id,category_id,name,description,category,tags,input_schema,version,published,health_status,call_count)
SELECT s.id,c.id,'demo_query','演示查询工具','database',ARRAY['demo','query'],
'{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}'::jsonb,'1.0.0',true,'online',42
FROM mcp_servers s CROSS JOIN tool_categories c WHERE s.name='Demo MCP Server' AND c.slug='database'
ON CONFLICT(server_id,name) DO UPDATE SET published=true,health_status='online',call_count=42,category_id=EXCLUDED.category_id,tags=EXCLUDED.tags;

INSERT INTO tool_versions(tool_id,version,input_schema,changelog,status,is_current,released_at)
SELECT t.id,'1.0.0',t.input_schema,'A16 demo release','published',true,NOW() FROM mcp_tools t WHERE t.name='demo_query'
ON CONFLICT(tool_id,version) DO UPDATE SET status='published',is_current=true;

INSERT INTO tool_permissions(tool_id,role_id,action)
SELECT t.id,r.id,a.action FROM mcp_tools t CROSS JOIN roles r CROSS JOIN (VALUES('read'),('call')) a(action)
WHERE t.name='demo_query' AND r.name IN ('platform_admin','tool_developer','agent_caller') ON CONFLICT DO NOTHING;

INSERT INTO tool_ratings(tool_id,user_id,rating,comment)
SELECT t.id,u.id,5,'A16 演示评分' FROM mcp_tools t CROSS JOIN users u WHERE t.name='demo_query' AND u.username='demo_agent'
ON CONFLICT(tool_id,user_id) DO UPDATE SET rating=5,comment='A16 演示评分';

UPDATE mcp_tools t SET average_rating=x.average,rating_count=x.count FROM
(SELECT tool_id,AVG(rating)::numeric(3,2) average,COUNT(*) count FROM tool_ratings GROUP BY tool_id)x WHERE t.id=x.tool_id;

INSERT INTO tool_adaptation_tasks(tool_id,task_type,status,source_url)
SELECT t.id,'openapi_import','success','https://example.com/demo-openapi.json' FROM mcp_tools t WHERE t.name='demo_query'
AND NOT EXISTS(SELECT 1 FROM tool_adaptation_tasks a WHERE a.tool_id=t.id AND a.source_url='https://example.com/demo-openapi.json');

INSERT INTO audit_logs(request_id,user_id,tool_id,server_id,tool_name,caller_role,duration_ms,status,http_status,params_digest)
SELECT 'a16-demo-request',u.id,t.id,s.id,t.name,'agent_caller',20,'success',200,repeat('0',64)
FROM users u CROSS JOIN mcp_tools t JOIN mcp_servers s ON s.id=t.server_id
WHERE u.username='demo_agent' AND t.name='demo_query' AND NOT EXISTS(SELECT 1 FROM audit_logs WHERE request_id='a16-demo-request');

INSERT INTO anomaly_alerts(alert_type,severity,tool_id,tool_name,message,status)
SELECT 'demo_alert','low',t.id,t.name,'A16 演示告警','open' FROM mcp_tools t WHERE t.name='demo_query'
AND NOT EXISTS(SELECT 1 FROM anomaly_alerts WHERE alert_type='demo_alert' AND tool_id=t.id);
