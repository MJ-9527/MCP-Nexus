# MCP-Nexus 运维手册：备份恢复与故障演练

> 本手册对应成员 C 任务 **C15：执行备份恢复与故障演练**。所有命令默认在 Git Bash / Linux shell 中执行，工作目录为项目根目录 `MCP-Nexus/`。

## 1. 备份策略

### 1.1 PostgreSQL 业务库备份

使用 `pg_dump` 定期导出业务数据。

```bash
# 进入容器执行备份（容器内已有 psql/pg_dump）
docker exec -t mcp-nexus-postgres pg_dump \
  -U mcp_user -d mcp_platform -Fc \
  > backups/mcp_platform_$(date +%Y%m%d_%H%M%S).dump

# 查看备份文件大小
ls -lh backups/
```

> 说明：`-Fc` 输出自定义格式，支持压缩和选择性恢复。

### 1.2 ClickHouse 分析库备份

ClickHouse 默认数据库为 `mcp_analytics`，使用 `clickhouse-backup` 或 SQL 导出。

```bash
# 示例：导出单表到 CSV（根据实际表名替换）
docker exec mcp-nexus-clickhouse clickhouse-client \
  --query "SELECT * FROM mcp_analytics.events FORMAT CSV" \
  > backups/mcp_analytics_events_$(date +%Y%m%d_%H%M%S).csv
```

生产环境建议使用 [clickhouse-backup](https://github.com/AlexAkulov/clickhouse-backup) 进行整库冷备。

### 1.3 配置与代码备份

```bash
# 备份运行时配置（.env 不提交仓库，需单独备份）
cp .env backups/env.$(date +%Y%m%d_%H%M%S)

# Git 已托管代码，定期推送分支即可
git push origin member-c
```

## 2. 恢复流程

### 2.1 PostgreSQL 恢复

**场景**：误删 `demo_customers` 表数据或整库损坏。

```bash
# 1. 停止依赖 postgres 的服务（避免写入）
cd deploy
docker compose stop demo-service gateway

# 2. 进入 postgres 容器恢复
docker exec -i mcp-nexus-postgres pg_restore \
  -U mcp_user -d mcp_platform --clean --if-exists \
  < backups/mcp_platform_YYYYMMDD_HHMMSS.dump

# 3. 重启服务
docker compose start demo-service gateway
```

> `--clean --if-exists` 会先清理已存在对象再恢复，适合整库恢复；单表恢复建议用 `pg_restore -t <table>`。

### 2.2 ClickHouse 恢复

```bash
# 恢复 CSV 到临时表，再 INSERT SELECT 回原表
docker exec -i mcp-nexus-clickhouse clickhouse-client \
  --query "INSERT INTO mcp_analytics.events FORMAT CSV" \
  < backups/mcp_analytics_events_YYYYMMDD_HHMMSS.csv
```

## 3. 故障演练

### 3.1 演练目标

| 场景 | 预期现象 | 验证手段 |
|---|---|---|
| PostgreSQL 不可用 | demo-service 数据库工具返回 503；网关健康检查显示 `postgres: unhealthy` | `curl http://localhost:8081/tools/query_customer/call` |
| demo-service 宕机 | skills-adapter 健康检查失败；网关依赖检测异常 | `docker compose ps`、`curl http://localhost:8082/health` |
| 单服务重启 | 服务自动恢复，依赖服务通过 `depends_on` 等待 | `docker compose up -d` 后观察状态 |
| 磁盘满导致 ClickHouse 写入失败 | 分析查询报错或响应变慢 | ClickHouse 日志、`docker logs mcp-nexus-clickhouse` |

### 3.2 PostgreSQL 故障注入与恢复

```bash
cd deploy

# 1. 模拟 PostgreSQL 宕机
docker compose stop postgres

# 2. 验证 demo-service 数据库工具返回 503
curl -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"region":"华东","limit":5}'
# 预期返回：{"error":"database unavailable"}

# 3. 验证网关健康检查识别依赖异常
curl http://localhost:8080/health
# 预期 data.dependencies 中 postgres 状态为 unhealthy

# 4. 恢复 PostgreSQL
docker compose start postgres

# 5. 等待 healthy 后再次验证
docker compose ps
curl -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"region":"华东","limit":5}'
```

### 3.3 demo-service 重启演练

```bash
cd deploy

# 停止 demo-service，skills-adapter 会因 depends_on 健康检查失败而不再 healthy
docker compose stop demo-service

# 观察 skills-adapter 状态
docker compose ps

# 重新启动并观察自动恢复
docker compose up -d demo-service
docker compose ps

# 验证 skills 工具仍可调用
curl -X POST http://localhost:8082/tools/calculate_vat/call \
  -H 'Content-Type: application/json' \
  -d '{"amount":100}'
```

### 3.4 强制删除客户数据并恢复（端到端备份有效性验证）

```bash
# 1. 记录删除前数据量
curl -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"limit":100}'

# 2. 执行备份
docker exec -t mcp-nexus-postgres pg_dump \
  -U mcp_user -d mcp_platform -Fc \
  > backups/before_delete.dump

# 3. 使用敏感工具删除一条数据（需 API Key）
curl -X POST http://localhost:8081/tools/delete_customer/call \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: demo-api-key' \
  -d '{"id":1}'

# 4. 验证数据已删除
curl -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"limit":100}'

# 5. 恢复备份
docker exec -i mcp-nexus-postgres pg_restore \
  -U mcp_user -d mcp_platform -t demo_customers --clean --if-exists \
  < backups/before_delete.dump

# 6. 验证数据已恢复
curl -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"limit":100}'
```

> 注意：`delete_customer` 是敏感工具，必须携带正确的 `X-API-Key`，否则返回 401 且不会执行下游删除。

## 4. 日志与链路排查

### 4.1 查看结构化日志

```bash
# demo-service 实时日志，每条请求包含 request_id
docker logs -f deploy-demo-service-1

# 通过 request_id 追踪一次完整调用链
curl -H 'X-Request-ID: trace-20260921-001' \
  -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"region":"华东","limit":5}'
```

### 4.2 指标检查

```bash
# demo-service 内存指标
curl http://localhost:8081/metrics
```

返回示例：

```json
{
  "requests_total": 42,
  "requests_by_status": { "200": 35, "400": 4, "503": 3 },
  "tool_calls_total": 30,
  "tool_calls_by_name": { "query_customer": 12, "query_sales": 10, "read_file": 5, "fetch_url": 3 },
  "errors_total": 7,
  "error_details": { "query_customer_db_unavailable": 3, "fetch_url_domain_not_allowed": 4 }
}
```

### 4.3 OpenTelemetry 链路（可选）

如需接入分布式链路，可在 `examples/demo-service/main.go` 中初始化 OTel SDK，将 `request_id` 作为 `trace_id` 注入。
当前版本已预留 `request_id` 上下文，接入步骤：

1. 引入 `go.opentelemetry.io/otel` 和 `go.opentelemetry.io/otel/exporters/otlp/otlptrace`。
2. 在 `main()` 中配置 TracerProvider（如导出到 Jaeger/OTel Collector）。
3. 在 `requestIDMiddleware` 中把 `request_id` 写入 Span attribute。

## 5. 日常检查清单

- [ ] `docker compose ps` 所有服务 `Status` 为 `(healthy)`
- [ ] 关键数据每日备份到 `backups/`
- [ ] 检查 `backups/` 磁盘占用，避免撑满
- [ ] 定期执行 3.2 / 3.3 演练并记录 RTO/RPO
- [ ] 敏感操作（删除）保留审计日志（已包含 request_id）
