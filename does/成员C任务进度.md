# 成员 C 任务进度

> 分支：`member-c`（已推送 origin）
> 负责目录：`examples/demo-service/`、`deploy/`、`db/`

## 一、任务完成情况

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 1 | 示例服务 `GET /health` + `POST /tools/query_sales/call` | ✅ 完成 | 抽出 `setupRouter()`，含 4 个单测，`go test` 通过 |
| 2 | 健康检查 Client（`GET {endpoint}/health`） | ✅ 完成 | 超时 5s + 测耗时，5 个单测通过 |
| 3 | 接口 `POST /api/servers/:id/health-check` | ✅ 完成（骨架） | handler + service 编排 + 路由已接，保存这步留 TODO |
| 4 | 计算并保存 `health_status` / `last_health_check_at` / `latency_ms` | ⚠️ 部分完成 | 计算 + 响应返回已做；**持久化待成员 A** |
| 5 | 扩展 Compose（postgres / demo-service / gateway） | ✅ 完成 | postgres + demo-service 已配；gateway 注释待接 |
| 6 | `seed.sql`（demo-db-server + query_sales） | ✅ 完成 | 幂等可重复执行（`WHERE NOT EXISTS`） |

## 二、交付文件

```text
examples/demo-service/
  main.go            # 抽出 setupRouter()，/health + /tools/query_sales/call
  main_test.go       # 4 个用例（online / 正常 / 400 / 空 body）
  Dockerfile         # 多阶段构建，含 curl+jq 供 compose healthcheck
  go.mod / go.sum    # module demo-service，gin v1.12.0

client/
  health_client.go   # HealthClient.Check()：GET {endpoint}/health + 超时 + 耗时
  health_client_test.go  # 5 个用例

service/
  health_check.go    # HealthCheckService.HealthCheck()：查 Server→探测→定状态
  health_check_test.go   # 4 个用例

handler/
  health_check.go    # POST /api/servers/:id/health-check 处理器

router/
  router.go          # 注册 health-check 路由

deploy/
  docker-compose.yml # 修正 environment/、注释符 #，postgres+demo-service

db/
  seed.sql           # demo-db-server + query_sales，幂等

cmd/server/main.go   # 恢复为网关原始入口（曾误被 demo-service 内容覆盖）
```

## 三、状态映射约定（已实现）

```
2xx        → online
非 2xx      → degraded
超时/连不上 → offline
```

对应 `service/health_check.go` 里的 `statusFromErr`。

## 四、阻塞项：依赖成员 A

1. **`UpdateHealthStatus(ctx, id, status, checkedAt)` 接口方法** —— 任务 4 的保存这步靠它。代码里已在
   `service/health_check.go` 留 `TODO(成员A)`，加好后取消注释即可。
2. **迁移 SQL 补齐** —— `001_create_mcp_servers.sql`、`002_create_mcp_tools.sql` 目前仍为空，
   `docker compose up -d` 无法建表，seed.sql 也无法执行。
3. **`.env.example`** —— compose 里 postgres 用了 `env_file: ../.env`，目前 `.env` 还没提供。

> 与成员 A 需对齐的契约（已写进 seed.sql 注释）：
> - id / server_id 用 `int64`（BIGINT），不是文档里的 VARCHAR(64)。
> - tags / input_schema 用 `JSONB`。
> - demo-service 的 endpoint 是 compose 网络名 `http://demo-service:8081`。

## 五、阻塞项：依赖成员 B

- **gateway 服务接入 compose** —— `deploy/docker-compose.yml` 里 gateway 段目前注释着，
  等成员 B 的网关代码就绪后取消注释即可。

## 六、接下来要做什么

**优先级排序：**

1. 【等成员 A】补上 `UpdateHealthStatus` → 取消 `TODO`，完成任务 4 持久化。
2. 【等成员 A】补迁移 SQL + `.env` → 跑通 `docker compose up -d`，验证验收链路：
   ```text
   PostgreSQL healthy → demo-service healthy → /health 返回成功 → 健康检查后 online
   ```
3. 【可选，不依赖别人】给健康检查加**定时任务**（每 30s/60s 一次）+ **启动时检查一次**
   （第一周任务.md 建议，成员 C 的 6 条任务里未强制，可作为加分项）。
4. 【等成员 B】gateway 接入 compose，打通完整链路。
5. **合并前自查**（团队成员通用要求）：
   ```powershell
   gofmt -w .
   go test ./...
   git status
   ```

## 七、当前验证状态

```text
examples/demo-service   go test -v ./...    → 4 个用例全部 PASS
主模块 client/ + service/  go test -v ./... → 9 个用例全部 PASS
```

> 注：主模块 `go test` 需先 `go mod tidy`（记得用 `GOPROXY=https://goproxy.cn,direct` 代理）。
