# 企业级 MCP 工具市场与智能体网关 - 第 1 周 API 接口文档

## 1. 文档说明

本文档基于《第一周任务.md》整理，聚焦第 1 周必须落地的最小 API 能力，目标是让项目先具备一条可运行的基础链路：

```
数据库初始化
→ 注册 MCP Server 和工具
→ 网关读取注册信息
→ Agent 获取工具列表
→ 网关检查工具健康状态
→ 为后续调用、权限和审计提供基础
```

本阶段重点不是全部功能，而是先统一接口定义、数据模型和基础调用链路，避免后续前后端联调时出现字段不一致和接口冲突。

---

## 2. 接口设计原则

### 2.1 通用约定

- 请求/响应均使用 JSON
- 所有接口统一返回结构
- 请求失败时返回统一错误格式
- 所有受保护接口在第 1 周可先预留 JWT 中间件，但不强制要求完全接入
- `request_id` 用于排查问题和链路追踪

### 2.2 统一响应格式

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_123456",
  "data": {
    "id": "server_001"
  }
}
```

#### 错误响应

```json
{
  "code": "SERVER_NOT_FOUND",
  "message": "MCP Server 不存在",
  "request_id": "req_abc123"
}
```

### 2.3 通用 HTTP 状态码

| 状态码 | 含义 |
|---|---|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 429 | 请求频率超限 |
| 500 | 服务器内部错误 |

### 2.4 统一错误码

| 错误码 | 含义 |
|---|---|
| INVALID_PARAMETER | 请求参数错误 |
| SERVER_NOT_FOUND | MCP Server 不存在 |
| TOOL_NOT_FOUND | 工具不存在 |
| SERVER_UNAVAILABLE | Server 不可用 |
| TOOL_OFFLINE | 工具已下线 |
| INTERNAL_ERROR | 系统内部错误 |
| UNAUTHORIZED | 未认证 |
| FORBIDDEN | 权限不足 |

---

## 3. 基础 URL

```text
http://localhost:8080
```

说明：

- `registry` 服务和 `gateway` 服务可以拆成两个独立服务
- 也可在 MVP 中先采用同一套后端服务，后续再拆分模块
- 本接口文档以统一网关/注册中心入口为例

---

## 4. 数据模型（第 1 周最小版本）

### 4.1 用户表 `users`

```sql
id               UUID / VARCHAR
username         VARCHAR(64)
password_hash    VARCHAR(255)
role             VARCHAR(32)
status           VARCHAR(16)
created_at       TIMESTAMP
updated_at       TIMESTAMP
```

说明：

- 角色可先简单支持：admin / developer / caller
- 第 1 周可以先预留字段，不要求立即完成全部权限细化

### 4.2 MCP Server 表 `mcp_servers`

```sql
id                  VARCHAR(64)
name                VARCHAR(128)
description         TEXT
endpoint            VARCHAR(255)
version             VARCHAR(32)
owner_id            VARCHAR(64)
status              VARCHAR(32)
health_status       VARCHAR(32)
last_health_check_at TIMESTAMP
created_at          TIMESTAMP
updated_at          TIMESTAMP
```

状态建议：

- `draft`：草稿
- `published`：已发布
- `offline`：已下线
- `unknown`：未知
- `online`：在线
- `unhealthy`：异常

### 4.3 工具表 `mcp_tools`

```sql
id               VARCHAR(64)
server_id        VARCHAR(64)
name             VARCHAR(128)
description      TEXT
category         VARCHAR(64)
tags             JSONB / TEXT[]
input_schema     JSONB
version          VARCHAR(32)
published        BOOLEAN
health_status    VARCHAR(32)
call_count       BIGINT
created_at       TIMESTAMP
updated_at       TIMESTAMP
```

说明：

- `published` 表示是否允许在市场和网关中被展示与调用
- `health_status` 可以用于网关判断是否可用
- `input_schema` 用于前端展示参数要求

### 4.4 工具版本表 `tool_versions`

```sql
id            VARCHAR(64)
tool_id       VARCHAR(64)
version       VARCHAR(32)
input_schema  JSONB
changelog     TEXT
status        VARCHAR(32)
created_at    TIMESTAMP
```

说明：

- 第 1 周可以先支持当前版本即可
- 版本管理可后续逐步完善

---

## 5. 注册中心 API

### 5.1 创建 MCP Server

#### 接口

```http
POST /api/servers
```

#### 请求体

```json
{
  "name": "demo-db-server",
  "description": "演示数据库 MCP Server",
  "endpoint": "http://demo-db-server:9001",
  "version": "1.0.0"
}
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1001",
  "data": {
    "id": "server_001",
    "name": "demo-db-server",
    "endpoint": "http://demo-db-server:9001",
    "status": "draft",
    "health_status": "unknown",
    "created_at": "2026-09-01T10:00:00Z"
  }
}
```

#### 失败响应

```json
{
  "code": "INVALID_PARAMETER",
  "message": "name 和 endpoint 是必填字段",
  "request_id": "req_1002"
}
```

---

### 5.2 查询所有 Server

#### 接口

```http
GET /api/servers
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| page | integer | 否 | 页码 |
| page_size | integer | 否 | 每页数量 |
| status | string | 否 | 过滤服务状态 |
| name | string | 否 | 按名称模糊查询 |

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1003",
  "data": {
    "items": [
      {
        "id": "server_001",
        "name": "demo-db-server",
        "description": "演示数据库 MCP Server",
        "endpoint": "http://demo-db-server:9001",
        "version": "1.0.0",
        "status": "draft",
        "health_status": "unknown"
      }
    ],
    "page": 1,
    "page_size": 10,
    "total": 1
  }
}
```

---

### 5.3 查询单个 Server

#### 接口

```http
GET /api/servers/{id}
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1004",
  "data": {
    "id": "server_001",
    "name": "demo-db-server",
    "description": "演示数据库 MCP Server",
    "endpoint": "http://demo-db-server:9001",
    "version": "1.0.0",
    "status": "published",
    "health_status": "online",
    "last_health_check_at": "2026-09-01T10:00:00Z"
  }
}
```

#### 失败响应

```json
{
  "code": "SERVER_NOT_FOUND",
  "message": "MCP Server 不存在",
  "request_id": "req_1005"
}
```

---

### 5.4 查询工具列表

#### 接口

```http
GET /api/tools
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| q | string | 否 | 关键词搜索 |
| category | string | 否 | 工具分类 |
| server_id | string | 否 | 按 Server 过滤 |
| published | boolean | 否 | 是否已发布 |
| page | integer | 否 | 页码 |
| page_size | integer | 否 | 每页数量 |

#### 示例请求

```http
GET /api/tools?q=数据库&category=database
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1006",
  "data": {
    "items": [
      {
        "id": "tool_001",
        "server_id": "server_001",
        "name": "query_db",
        "description": "查询脱敏业务数据库",
        "category": "database",
        "tags": ["db", "report"],
        "version": "1.0.0",
        "published": true,
        "health_status": "online",
        "call_count": 12
      }
    ],
    "page": 1,
    "page_size": 10,
    "total": 1
  }
}
```

---

### 5.5 查询单个工具

#### 接口

```http
GET /api/tools/{id}
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1007",
  "data": {
    "id": "tool_001",
    "server_id": "server_001",
    "name": "query_db",
    "description": "查询脱敏业务数据库",
    "category": "database",
    "tags": ["db", "report"],
    "input_schema": {
      "type": "object",
      "properties": {
        "sql": {
          "type": "string",
          "description": "查询 SQL"
        }
      },
      "required": ["sql"]
    },
    "version": "1.0.0",
    "published": true,
    "health_status": "online",
    "call_count": 12
  }
}
```

---

### 5.6 发布工具

#### 接口

```http
POST /api/tools/{id}/publish
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "工具已发布",
  "request_id": "req_1008",
  "data": {
    "id": "tool_001",
    "published": true,
    "updated_at": "2026-09-01T11:00:00Z"
  }
}
```

#### 失败响应

```json
{
  "code": "TOOL_NOT_FOUND",
  "message": "工具不存在",
  "request_id": "req_1009"
}
```

---

### 5.7 下线工具

#### 接口

```http
POST /api/tools/{id}/offline
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "工具已下线",
  "request_id": "req_1010",
  "data": {
    "id": "tool_001",
    "published": false,
    "updated_at": "2026-09-01T11:05:00Z"
  }
}
```

---

### 5.8 健康检查

#### 接口

```http
POST /api/servers/{id}/health-check
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1011",
  "data": {
    "server_id": "server_001",
    "health_status": "online",
    "latency_ms": 12,
    "checked_at": "2026-09-01T10:00:00Z"
  }
}
```

#### 失败响应

```json
{
  "code": "SERVER_UNAVAILABLE",
  "message": "MCP Server 无法访问",
  "request_id": "req_1012"
}
```

---

## 6. 网关 API

网关是 Agent 调用工具的唯一入口，负责统一路由和转发。第 1 周重点实现基础能力。

### 6.1 网关健康检查

#### 接口

```http
GET /health
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1013",
  "data": {
    "status": "healthy",
    "timestamp": "2026-09-01T10:00:00Z"
  }
}
```

---

### 6.2 获取当前可用工具列表

#### 接口

```http
GET /mcp/tools
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1014",
  "data": [
    {
      "id": "tool_001",
      "name": "query_db",
      "description": "查询脱敏业务数据库",
      "category": "database",
      "server_id": "server_001",
      "published": true,
      "health_status": "online"
    }
  ]
}
```

说明：

- 该接口只返回已发布、健康、可调用的工具
- 第 1 周先做最基础的列表展示

---

### 6.3 调用指定工具

#### 接口

```http
POST /mcp/tools/{toolName}/call
```

#### 请求体

```json
{
  "arguments": {
    "sql": "SELECT * FROM orders WHERE customer_id = 1001"
  }
}
```

#### 成功响应

```json
{
  "code": "SUCCESS",
  "message": "ok",
  "request_id": "req_1015",
  "data": {
    "tool": "query_db",
    "result": [
      {
        "order_id": 1001,
        "status": "paid"
      }
    ]
  }
}
```

#### 失败响应

```json
{
  "code": "TOOL_OFFLINE",
  "message": "工具当前不可用，已下线或不健康",
  "request_id": "req_1016"
}
```

说明：

- 第 1 周先支持最基础的 HTTP 转发
- 复杂权限校验、限流、审计记录可放到第 2 周实现

---

## 7. 第 1 周建议的最小接口清单

| 模块 | 接口 | 目的 |
|---|---|---|
| Server 管理 | `POST /api/servers` | 注册 MCP Server |
| Server 管理 | `GET /api/servers` | 查询 Server 列表 |
| Server 管理 | `GET /api/servers/{id}` | 查询单个 Server |
| Tool 管理 | `GET /api/tools` | 查询工具列表 |
| Tool 管理 | `GET /api/tools/{id}` | 查询单个工具 |
| Tool 管理 | `POST /api/tools/{id}/publish` | 发布工具 |
| Tool 管理 | `POST /api/tools/{id}/offline` | 下线工具 |
| 健康检查 | `POST /api/servers/{id}/health-check` | 检查服务状态 |
| 网关健康 | `GET /health` | 网关可用性检查 |
| 网关工具 | `GET /mcp/tools` | 获取可用工具列表 |
| 网关调用 | `POST /mcp/tools/{toolName}/call` | 调用工具 |

---

## 8. 第 1 周开发约束

为了保证开发顺利推进，第 1 周建议遵守以下约束：

1. 先定接口、再写代码
2. 所有字段命名必须统一
3. 统一返回结构必须保证一致
4. 不要在第 1 周引入复杂鉴权和审计逻辑
5. 先保证“能注册、能查、能健康检查、能调用”这条链路跑通
6. 后续功能按“第 2 周安全闭环、第 3 周市场、第 4 周交付”逐步扩展

---

## 9. 接口开发建议

### 9.1 后端结构建议

```text
gateway/
├── router/
├── handler/
├── service/
├── repository/
├── client/
├── middleware/
└── model/
```

### 9.2 接口验证建议

- 确认 `POST /api/servers` 能保存数据
- 确认 `GET /mcp/tools` 能从数据库拿到已发布工具
- 确认 `POST /api/servers/{id}/health-check` 能更新 `health_status`
- 确认 `POST /mcp/tools/{toolName}/call` 能转发到下游 MCP Server
- 确认所有失败场景都返回统一错误结构

---

## 10. 结论

第 1 周的核心目标是定义清晰的 API 契约，并用最小功能实现“注册中心 + 网关 + 工具列表 + 健康检查 + 基础调用链路”。这一步完成后，后续权限、限流、审计和前端市场都会比较容易接上。

如果后续需要，可以进一步扩展为：

- 用户认证接口
- RBAC 权限接口
- Redis 限流接口
- 审计日志查询接口
- OpenAPI 导入接口

这些都属于第 2 周及之后的功能补充。
