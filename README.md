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
```

### 6. 停止

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
