<<<<<<< HEAD
# MCP-Nexus — 企业级 MCP 工具市场与智能体网关

面向企业内部 AI Agent 的 MCP 工具市场与智能体网关 MVP。

统一管理 MCP Server 和工具，提供工具发现、JWT 认证、RBAC 鉴权、调用代理、限流、审计和基础统计能力。

---

## 环境要求

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.21+ | 后端服务 |
| PostgreSQL | 14+ | 元数据存储 |
| Redis | 7+ | 限流（可选，MVP 暂用内存） |
| Docker + docker-compose | 20+ | 一键启动完整环境 |
| PowerShell 5.1+ / 7+ | — | 接口联调脚本 |

---

## 项目结构

```
MCP-Nexus/
├── cmd/server/main.go          # 入口（自动降级内存存储）
├── config/
│   ├── config.go               # 环境变量加载
│   └── database.go             # PostgreSQL 连接池
├── db/
│   ├── migrations/             # SQL 迁移（按序执行）
│   │   ├── 001_create_mcp_servers.sql
│   │   ├── 002_create_mcp_tools.sql
│   │   ├── 003_create_registry_indexes.sql
│   │   ├── 004_create_users_roles.sql
│   │   └── 005_create_tool_permissions.sql
│   └── seed.sql                # 演示数据
├── deploy/
│   ├── docker-compose.yml      # postgres + redis + clickhouse + demo + gateway
│   └── Dockerfile              # 网关容器镜像
├── examples/demo-service/      # 示例 MCP Server（C 成员交付）
│   ├── main.go
│   └── Dockerfile
├── handler/                    # HTTP 处理层
│   ├── common.go               # 统一响应 helper + Health
│   ├── server_register.go
│   ├── server_query.go
│   ├── server_health.go
│   ├── server_status.go
│   ├── tool_register.go
│   ├── tool_query.go
│   ├── tool_publish.go
│   ├── mcp_gateway.go          # B 成员：MCP 网关入口
│   ├── auth.go                 # B 成员：登录/注册
│   ├── permission.go           # A 成员：权限管理
│   └── rbac.go
├── middleware/
│   ├── request_id.go           # 全局 request_id
│   ├── context.go              # Bearer 提取
│   ├── jwt.go                  # B 成员：JWT 生成/验证/中间件
│   └── audit.go                # B 成员：审计日志
├── client/
│   ├── health_client.go        # 健康检查
│   └── mcp_client.go           # B 成员：下游 MCP HTTP 客户端
├── model/model.go              # 所有数据模型
├── repository/
│   ├── server.go               # ServerRepository 接口
│   ├── tool.go                 # ToolRepository 接口
│   ├── user.go                 # UserRepository 接口
│   ├── tool_permission.go      # ToolPermissionRepository 接口
│   ├── memory_server.go        # 内存实现（已有）
│   ├── postgres_server.go      # A 成员：PostgreSQL Server
│   ├── postgres_tool.go        # A 成员：PostgreSQL Tool
│   ├── postgres_user.go        # A 成员：PostgreSQL User
│   ├── postgres_tool_permission.go  # A 成员：PostgreSQL 权限
│   └── memory_fallback.go      # 内存实现（ fallback，无 PG 时使用）
├── service/
│   ├── server.go / register.go / query.go / status.go / health.go
│   ├── tool.go / register.go / query.go / publish.go
│   ├── user.go
│   ├── permission.go           # A 成员：权限服务
│   └── mcp_gateway.go          # B 成员：网关服务
├── router/router.go            # 路由注册（PG / 内存双模式）
├── tests/
│   ├── integration_test.go     # 16 个集成测试（D 成员）
│   └── test-api.ps1            # PowerShell 联调脚本（D 成员）
├── .env.example                # 环境变量模板
└── README.md
```

---

## 快速启动

### 方式一：Docker Compose（推荐，含完整环境）

```powershell
cd deploy
docker compose up -d
# 等待 PostgreSQL 就绪
docker compose logs -f postgres
# 另开终端：初始化数据库
docker exec -i mcp-nexus-postgres psql -U mcp_nexus -d mcp_platform -f ../db/migrations/001_create_mcp_servers.sql
docker exec -i mcp-nexus-postgres psql -U mcp_nexus -d mcp_platform -f ../db/migrations/002_create_mcp_tools.sql
docker exec -i mcp-nexus-postgres psql -U mcp_nexus -d mcp_platform -f ../db/migrations/003_create_registry_indexes.sql
docker exec -i mcp-nexus-postgres psql -U mcp_nexus -d mcp_platform -f ../db/migrations/004_create_users_roles.sql
docker exec -i mcp-nexus-postgres psql -U mcp_nexus -d mcp_platform -f ../db/migrations/005_create_tool_permissions.sql
docker exec -i mcp-nexus-postgres psql -U mcp_nexus -d mcp_platform -f ../db/seed.sql
```

### 方式二：本地运行（无需 PostgreSQL，自动使用内存存储）

```powershell
cd ..
# 启动示例服务
cd examples/demo-service; go run main.go   # :9001
# 另开终端，启动网关
cd ..; go run ./cmd/server/                 # :8080
```

网关会自动检测 PostgreSQL 连接，连接失败时使用内存存储，不影响功能演示。

---

## Windows PowerShell 调用差异

| 操作 | PowerShell 7+ / GoLand | Windows CMD | Git Bash |
|------|------------------------|-------------|----------|
| 运行脚本 | `.\\tests\\test-api.ps1` | `powershell .\\tests\\test-api.ps1` | `powershell ./tests/test-api.ps1` |
| 读环境变量 | `$env:PORT` | `%PORT%` | `$PORT` |
| JSON 解析 | `ConvertFrom-Json` | — | `jq` |
| HTTP 请求 | `Invoke-RestMethod` | `curl` | `curl` / `jq` |

---

## 接口总览

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/health` | 健康检查 | 无 |
| POST | `/api/auth/login` | 用户登录，返回 JWT | 无 |
| POST | `/api/auth/register` | 注册用户 | 无 |
| GET | `/api/auth/me` | 当前用户信息 | 可选 |
| GET/POST | `/api/servers` | Server 列表/注册 | JWT |
| GET | `/api/servers/:id` | Server 详情 | JWT |
| POST | `/api/servers/:id/activate` | 激活 Server | JWT |
| POST | `/api/servers/:id/offline` | 下线 Server | JWT |
| POST | `/api/servers/:id/health-check` | 健康检查 | JWT |
| GET/POST | `/api/tools` | Tool 列表/注册 | JWT |
| GET | `/api/tools/:id` | Tool 详情 | JWT |
| POST | `/api/tools/:id/publish` | 发布 Tool | JWT |
| POST | `/api/tools/:id/offline` | 下线 Tool | JWT |
| POST/GET/DELETE | `/api/permissions/*` | 权限管理 | JWT |
| GET | `/api/audit/logs` | 审计日志 | JWT |
| GET | `/mcp/tools` | 可用工具列表（网关） | 无 |
| POST | `/mcp/tools/:toolName/call` | 调用工具（网关） | 无 |

---

## 接口示例

### 登录

```powershell
$body = @{ username = "admin"; password = "password123" } | ConvertTo-Json
$resp = Invoke-RestMethod http://localhost:8080/api/auth/login -Method Post -ContentType "application/json" -Body $body
$token = $resp.data.token
```

### 注册 Server

```powershell
$body = @{ name = "demo-db-server"; endpoint = "http://localhost:9001"; version = "1.0.0" } | ConvertTo-Json
$headers = @{ Authorization = "Bearer $token" }
Invoke-RestMethod http://localhost:8080/api/servers -Method Post -ContentType "application/json" -Headers $headers -Body $body
```

### 调用工具

```powershell
$body = @{ arguments = @{ month = "2026-08"; region = "华东" } } | ConvertTo-Json
Invoke-RestMethod http://localhost:8080/mcp/tools/query_sales/call -Method Post -ContentType "application/json" -Body $body
```

---

## 错误码规范

| HTTP 状态码 | code 字段 | 含义 |
|-------------|-----------|------|
| 200 | `OK` | 成功 |
| 400 | `INVALID_PARAMETER` | 请求参数错误 |
| 401 | `UNAUTHORIZED` | 未认证 |
| 403 | `FORBIDDEN` | 权限不足 |
| 404 | `SERVER_NOT_FOUND` / `TOOL_NOT_FOUND` | 资源不存在 |
| 409 | `SERVER_ALREADY_EXISTS` / `CONFLICT` | 资源冲突 |
| 429 | `RATE_LIMITED` | 请求频率超限 |
| 500 | `INTERNAL_ERROR` / `SERVER_UNAVAILABLE` / `TOOL_OFFLINE` | 服务器内部错误 |

所有响应均包含 `request_id` 字段，用于链路追踪。

---

## 测试

### 单元测试

```powershell
go test -buildvcs=false ./...
```

### 集成测试（需先启动网关）

```powershell
# 启动网关（端口 18080）
go run ./cmd/server/

# 新开终端运行测试
go test -buildvcs=false -tags=integration -v ./tests/

# 或指定地址
$env:TEST_BASE_URL = "http://localhost:18080"
go test -buildvcs=false -tags=integration -v ./tests/
```

### PowerShell 联调脚本

```powershell
.\tests\test-api.ps1
.\tests\test-api.ps1 -BaseURL "http://localhost:8080"
```

---

## 最小演示链路

```
1. go run ./cmd/server/              # 启动网关 (:8080)
2. cd examples/demo-service && go run main.go   # 启动示例服务 (:9001)
3. 登录获取 Token
4. POST /api/servers 注册 demo-db-server
5. POST /api/servers/:id/health-check 健康检查
6. GET /mcp/tools 查看可用工具
7. POST /mcp/tools/query_sales/call 调用工具
```

---

## 团队分工

| 角色 | 负责模块 |
|------|----------|
| A — 注册中心 & PostgreSQL | 数据模型、PostgreSQL Repository、User/Tool/Permission CRUD、Seed |
| B — 网关 & 安全 | MCP 网关路由、JWT 认证、RBAC、审计日志、MCP HTTP Client |
| C — 基础设施 & 示例服务 | demo-service、Docker Compose、Seed 数据、健康检查 |
| **D — 测试 & 文档 & 集成** | **API 契约、集成测试、PowerShell 脚本、README、验收报告** |

---

## 开发规范

```powershell
# 合并前必跑
gofmt -w .
go test -buildvcs=false ./...
go vet -buildvcs=false ./...
git status
```

---

## 版本信息

- **版本**: v0.2.0-MVP
- **构建日期**: 2026-09-17
- **Go 版本**: 1.21+
=======
# MCP-Nexus

本项目建设一个面向企业内部 AI Agent 的 MCP 工具市场与智能体网关，统一管理 MCP Server 和工具，提供工具发现、权限控制、调用代理、限流、审计和基础统计能力。

## 快速启动

### 1. 前置要求

- Docker + Docker Compose
- Go 1.26+（开发/调试网关）
- Git Bash / Linux shell（以下命令使用 POSIX 语法）

### 2. 克隆与配置

```bash
git clone <repo-url>
cd MCP-Nexus
cp .env.example .env
```

`.env.example` 已包含本地开发默认值，通常无需修改。

### 3. 一键启动核心环境

```bash
cd deploy
docker compose up -d --build
```

启动的服务：

| 服务 | 容器内地址 | 宿主机端口 | 说明 |
|---|---|---|---|
| PostgreSQL | `postgres:5432` | `15432` | 业务数据库 + 初始化迁移 |
| Redis | `redis:6379` | `16379` | 缓存/限流 |
| ClickHouse | `clickhouse:8123` | `18123` | 分析数据 |
| demo-service | `demo-service:8081` | `8081` | 示例 MCP Server（C5-C7） |
| skills-adapter | `demo-skills:8082` | `8082` | Skills 示例（C12） |
| gateway | `gateway:8080` | `8080` | 网关入口 |

### 4. 等待全部 healthy

```bash
docker compose ps
```

所有服务 `Status` 列应显示 `(healthy)`。

### 5. 验证

```bash
# 网关健康（含依赖状态）
curl http://localhost:8080/health

# demo-service 健康
curl http://localhost:8081/health

# 查询脱敏客户数据
curl -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"region":"华东","limit":5}'

# Skills 计算含税价
curl -X POST http://localhost:8082/tools/calculate_vat/call \
  -H 'Content-Type: application/json' \
  -d '{"amount":100}'

# demo-service 指标（C14）
curl http://localhost:8081/metrics
```

### 6. 查看结构化日志（C14）

每条请求日志包含 `request_id`、`method`、`path`、`status`、`duration_ms`、`response_size`：

```bash
docker logs -f deploy-demo-service-1
```

传入自定义 `X-Request-ID` 可在网关、demo-service、skills-adapter 之间串联一次调用链：

```bash
curl -H 'X-Request-ID: trace-20260921-001' \
  -X POST http://localhost:8081/tools/query_customer/call \
  -H 'Content-Type: application/json' \
  -d '{"region":"华东","limit":5}'
```

### 7. 停止

```bash
docker compose down -v
```

`-v` 会删除数据卷，下次启动为干净环境。

## 目录说明

```text
.
├── cmd/server/          # 网关入口
├── cmd/openapi-gen/     # OpenAPI → MCP 生成器 CLI（C10）
├── adapter/openapi/     # OpenAPI 解析/校验/生成库
├── client/              # 下游健康检查客户端
├── service/             # 业务编排（含健康检查）
├── handler/             # HTTP 接口
├── router/              # 路由注册
├── repository/          # 数据访问
├── examples/
│   ├── demo-service/    # 示例 MCP Server（C5-C7）
│   ├── openapi/         # OpenAPI 示例（C9）与失败示例（C11）
│   └── skills/          # Skills 示例包（C12）
├── deploy/
│   └── docker-compose.yml   # 一键部署（C13）
├── docs/
│   └── ops-runbook.md   # 备份恢复与故障演练（C15）
├── db/migrations/       # 数据库迁移与种子数据
└── does/                # 任务文档
```

## 常见问题

### 端口冲突

检查 `.env` 或 `deploy/docker-compose.yml` 中的端口映射，修改后重启：

```bash
docker compose down -v
docker compose up -d
```

### 数据库初始化失败

确认 `db/migrations/` 中有成员 A 的建表脚本（`001_*.sql`、`002_*.sql`）。`006_seed.sql` 和 `007_create_demo_business.sql` 依赖这些表。

### 国内镜像拉取慢

已使用 alpine 小镜像并配置 Go 代理 `GOPROXY=https://goproxy.cn,direct`。如仍失败，可配置 Docker 镜像源或本机构建二进制。
>>>>>>> origin/pull-request
