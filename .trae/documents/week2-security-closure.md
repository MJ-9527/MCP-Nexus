# 第 2 周实施方案：安全闭环（JWT + RBAC + Redis 限流 + ClickHouse 审计 + 3 示例 MCP Server）

## Context

第 1 周已完成注册中心、网关代理骨架、健康检查、定时检查、Docker Compose 三件套。但权限是 MockClient 通配放行（`anonymous`/`admin` 全开），无认证、无限流、无审计。第 2 周目标按需求文档 §4.1/§4.2/§4.4/§4.8/§9 完成"安全调用闭环"，让"合法用户能调、无权限被拒、敏感调用有日志、超限返回 429"可演示。

用户已确认 4 个决策：
- RBAC **网关内部实现**（不拆独立微服务）
- 限流**按角色**（Agent → QPS）
- 审计**直接上 ClickHouse**（一步到位）
- 3 个示例 MCP Server **三个独立二进制**

## 总体架构

```
Agent → [Auth: JWT 校验] → [Permission: RBAC] → [RateLimit: Redis 令牌桶] → Handler → [Audit: 异步写 ClickHouse]
                              ↓
                     permission.PermissionClient（内部实现，查 PostgreSQL role_tools 表）
```

## 一、依赖新增（go.mod）

```
go get github.com/golang-jwt/jwt/v5
go get github.com/redis/go-redis/v9
go get github.com/ClickHouse/clickhouse-go/v2
go get golang.org/x/crypto/bcrypt
```

## 二、数据模型与迁移

**新文件** `db/migrations/003_create_users_roles.sql`：
- `users`（id, username UNIQUE, password_hash, role, status, created_at, updated_at）— 复用 [model.User](file:///d:/Learn/MCP-Nexus/MCP-Nexus/model/model.go#L14-L22)
- `role_tools`（role VARCHAR, tool_name VARCHAR, PRIMARY KEY(role, tool_name)）

**新文件** `db/migrations/004_create_audit_logs.sql`（ClickHouse DDL，由 clickhouse 容器 initdb 执行）：
- `audit_logs`（request_id String, caller_id Int64, role String, tool_name String, server_id Int64, args_summary String, called_at DateTime, latency_ms Int32, success UInt8, http_status Int32, reject_reason String, cost_estimate Float32）ENGINE MergeTree

**新文件** `db/seed.sql`：插入 admin/developer/agent 三个用户（bcrypt hash 预生成）、demo-db-server、query_sales 工具、role_tools 映射。

## 三、认证（JWT 登录）

**新文件 `service/auth_service.go`**：
- `AuthService{ users repository.UserRepository; jwtSecret string; jwtTTL time.Duration }`
- `Login(ctx, username, password) (*TokenResponse, error)`：查 user → bcrypt.CompareHashAndPassword → 签 JWT（claims: user_id/role/exp，hs256）
- `ParseToken(tokenStr) (*Claims, error)`：jwt.ParseWithClaims

**新文件 `repository/user.go` + `postgres_user.go`**：
- `UserRepository` 接口：FindByUsername
- PG 实现：`SELECT id, username, password_hash, role, status FROM users WHERE username=$1`

**新文件 `handler/auth.go`**：
- `POST /api/auth/login` → 校验 → 返回 `{token, expires_at, user:{id,role}}`

**新文件 `middleware/auth.go`**：
- 从 `Authorization: Bearer <token>` 解析 JWT → `c.Set("user_id", ...)` + `c.Set("role", ...)`
- 失败返回 401 `UNAUTHORIZED`
- 复用 [respond.go](file:///d:/Learn/MCP-Nexus/MCP-Nexus/handler/respond.go#L28) 的 respondErrorCode

## 四、RBAC（网关内部实现）

**新文件 `repository/permission.go` + `postgres_permission.go`**：
- `PermissionRepository` 接口：`FindToolsByRole(ctx, role) ([]string, error)`
- PG 实现：`SELECT tool_name FROM role_tools WHERE role=$1`
- 同时实现 `permission.PermissionClient` 接口的适配器（包一层把 PermissionRepository 转成 PermissionClient），让 [proxy_service.go](file:///d:/Learn/MCP-Nexus/MCP-Nexus/service/proxy_service.go#L46) 不用改

**新文件 `service/permission_service.go`**：
- `PermissionService{ repo repository.PermissionRepository }`
- `GetAllowedToolNamesByRole(ctx, role) ([]string, error)` — 实现 permission.PermissionClient 接口
- 额外 `GET /api/permission/tools?role=xxx` 接口（兼容现有 [permission/client.go](file:///d:/Learn/MCP-Nexus/MCP-Nexus/permission/client.go#L33) 真实 HTTP 客户端，但内部直接调 repo）

**新文件 `middleware/permission.go`**（可选，或直接复用 proxy_service 内的 RBAC）：
- 网关 `/mcp/tools/:toolName/call` 路由的权限校验已在 [proxy_service.go#L83](file:///d:/Learn/MCP-Nexus/MCP-Nexus/service/proxy_service.go#L83) 完成，第2周只需把 MockClient 替换为真实 PermissionService

**router 改动** [router.go#L28-L30](file:///d:/Learn/MCP-Nexus/MCP-Nexus/router/router.go#L28)：
- 删除 MockClient 通配放行
- 注入 `permissionRepo := repository.NewPostgresPermissionRepository(pool)` → `permissionService := service.NewPermissionService(permissionRepo)`
- `NewProxyService(serverRepo, toolRepo, permissionService)` — permissionService 实现 PermissionClient 接口
- `/mcp/*` 路由组挂 `middleware.Auth(jwtSecret)` + `middleware.RateLimit(rdb, limiterCfg)`

## 五、Redis 限流（令牌桶，按角色）

**新文件 `middleware/rate_limit.go`**：
- `RateLimit(rdb *redis.Client, cfg map[string]int) gin.HandlerFunc`
- 按 `c.Get("role")` 取角色 → 查 QPS 配置（默认 10 QPS）
- 令牌桶：Redis INCR + EXPIRE，超限返回 429 `RATE_LIMITED`
- key: `ratelimit:role:{role}:{minute_bucket}`

**新文件 `service/limiter.go`** 或直接在 middleware 内实现。

## 六、ClickHouse 审计

**新文件 `repository/audit.go` + `clickhouse_audit.go`**：
- `AuditRepository` 接口：`Record(ctx, *AuditLog) error`
- ClickHouse 实现：`INSERT INTO audit_logs (request_id, caller_id, role, tool_name, ...) VALUES (...)`
- 批量异步写入：channel 缓冲 + 定时 flush

**新文件 `service/audit_service.go`**：
- `AuditService{ repo AuditRepository; queue chan *AuditLog }`
- `RecordAsync(ctx, log)` — 非阻塞推 channel
- 后台 goroutine 批量消费，500ms 或 100 条 flush 一次
- `Start(ctx)` — main 中 `go auditService.Start(ctx)`

**新文件 `model/audit.go`**：
- `AuditLog` struct，字段对齐 ClickHouse schema

**proxy_service 改动**：
- 注入 `audit *AuditService`
- `CallTool` 末尾：记录 caller_id/role/tool_name/latency_ms/success/reject_reason，调 `audit.RecordAsync`
- 参数脱敏：只存参数 JSON 的 SHA256 摘要前 16 位 + 长度，按需求 §4.8

**新接口 `GET /api/audit/logs`**（[handler/audit.go](file:///d:/Learn/MCP-Nexus/MCP-Nexus/handler/audit.go)）：
- 支持按 caller_id/tool_name/success 过滤，分页查询

## 七、3 个示例 MCP Server（三个独立二进制）

每个都遵循 [demo-service](file:///d:/Learn/MCP-Nexus/MCP-Nexus/examples/demo-service/main.go) 模式：`/health` + `/tools/{name}/call`

**新文件 `examples/file-server/main.go`**：
- 工具：`read_file`（参数 path，返回文件内容片段）
- 工具：`list_dir`（参数 dir，返回目录列表）
- 内置一个临时目录限制访问范围

**新文件 `examples/http-server/main.go`**：
- 工具：`http_get`（参数 url，转发 GET 请求返回 status/body 摘要）
- 工具：`http_post`（参数 url, body）
- 限制只允许 example.com 等白名单域名（防 SSRF）

**新文件 `examples/db-server/main.go`**：
- 工具：`query_sales`（复用 demo-service 逻辑，返回脱敏销售数据）
- 工具：`query_inventory`（返回库存数据）

**Dockerfile 改动** [deploy/Dockerfile](file:///d:/Learn/MCP-Nexus/MCP-Nexus/deploy/Dockerfile)：加 3 个 `go build`

**docker-compose 改动** [deploy/docker-compose.yml](file:///d:/Learn/MCP-Nexus/MCP-Nexus/deploy/docker-compose.yml)：
- 加 `clickhouse` 服务（image: clickhouse/clickhouse-server:24-alpine，挂载 `db/migrations/004_create_audit_logs.sql` 到 /docker-entrypoint-initdb.d）
- 加 `redis` 服务（image: redis:7-alpine，healthcheck）
- 加 `file-server`、`http-server`、`db-server` 三个容器（端口 9002/9003/9004）
- gateway 依赖加 redis + clickhouse

## 八、config 扩展

[config/config.go](file:///d:/Learn/MCP-Nexus/MCP-Nexus/config/config.go)：
- Config struct 加：`JWTSecret string`、`JWTTTL time.Duration`、`RedisAddr string`、`ClickHouseAddr string`
- Load 从 env 读取：`JWT_SECRET`、`JWT_TTL`、`REDIS_ADDR`、`CLICKHOUSE_ADDR`

[.env.example](file:///d:/Learn/MCP-Nexus/MCP-Nexus/.env.example) 同步更新。

## 九、main.go 改动

[cmd/server/main.go](file:///d:/Learn/MCP-Nexus/MCP-Nexus/cmd/server/main.go)：
- 创建 redis client、clickhouse client、userRepo、permissionRepo、auditRepo
- 启动 auditService 的异步消费 goroutine
- 注入到 router

## 十、验证方案

1. **单元测试**：
   - `service/auth_service_test.go`：bcrypt + JWT 签发/解析
   - `service/permission_service_test.go`：角色→工具列表
   - `middleware/rate_limit_test.go`：用 miniredis 验证 429

2. **端到端**：
   ```
   docker compose up -d
   → POST /api/auth/login (admin/admin123) → 拿 JWT
   → POST /api/tools/:id/publish（带 JWT）
   → GET /mcp/tools（带 JWT） → 返回工具列表
   → POST /mcp/tools/query_sales/call（带 JWT） → 成功
   → 不带 JWT 调 /mcp/tools → 401
   → 用 agent 角色调敏感工具 delete_customer → 403
   → 连续刷调用 → 429
   → SELECT * FROM audit_logs → 有完整调用记录
   ```

3. **编译验证**：`go build ./... && go vet ./... && go test ./...`

## 文件清单（新增/修改）

**新增**：
- db/migrations/003_create_users_roles.sql
- db/migrations/004_create_audit_logs.sql
- db/seed.sql
- repository/user.go, postgres_user.go
- repository/permission.go, postgres_permission.go
- repository/audit.go, clickhouse_audit.go
- service/auth_service.go, auth_service_test.go
- service/permission_service.go, permission_service_test.go
- service/audit_service.go
- middleware/auth.go
- middleware/rate_limit.go
- model/audit.go
- handler/auth.go, audit.go
- examples/file-server/main.go
- examples/http-server/main.go
- examples/db-server/main.go

**修改**：
- go.mod（加 4 个依赖）
- config/config.go（加 4 个字段）
- .env.example
- router/router.go（注入新 service，/mcp/* 挂中间件）
- service/proxy_service.go（注入 audit，末尾记审计）
- handler/proxy.go（去掉 X-Role，改从 context 取 role）
- deploy/docker-compose.yml（加 clickhouse/redis/3 示例服务）
- deploy/Dockerfile（加 3 个 go build）
- cmd/server/main.go（创建 redis/ch client，启动 audit goroutine）
