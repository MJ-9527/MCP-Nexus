-- Docker 网关必须通过 Compose 服务名访问演示服务，localhost 会指向网关容器自身。
UPDATE mcp_servers
SET endpoint = 'http://demo-service:8081',
    status = 'active',
    health_status = 'unknown',
    updated_at = NOW()
WHERE name = 'demo-db-server'
  AND endpoint <> 'http://demo-service:8081';
