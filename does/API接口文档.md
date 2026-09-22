# 企业级 MCP 工具市场与智能体网关 -- API 接口文档

> **版本**：v1.0-MVP
> **基础地址**：https://api.mcp-gateway.internal（演示环境）
> **传输协议**：HTTPS / HTTP
> **认证方式**：JWT Bearer Token
> **数据格式**：JSON (UTF-8)

---

## 目录

1. [通用规范](#1-通用规范)
2. [认证接口](#2-认证接口)
3. [MCP 网关接口](#3-mcp-网关接口)
4. [工具市场接口](#4-工具市场接口)
5. [权限管理接口](#5-权限管理接口)
6. [OpenAPI 导入接口](#6-openapi-导入接口)
7. [审计日志接口](#7-审计日志接口)
8. [统计分析接口](#8-统计分析接口)
9. [系统接口](#9-系统接口)
10. [统一错误格式](#10-统一错误格式)
11. [角色与权限矩阵](#11-角色与权限矩阵)
12. [限流策略](#12-限流策略)

---

## 2. 认证接口

### 2.1 用户登录

**接口**：POST /api/auth/login

**权限**：无需认证

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| username | string | 是 | 用户名，3-64 字符 |
| password | string | 是 | 密码，最少 8 位 |

\\json
{
  "username": "agent_dev_01",
  "password": "SecurePass123!"
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "message": "login success",
  "request_id": "req-login-001",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 3600,
    "user": {
      "id": "usr-8f3a2b",
      "username": "agent_dev_01",
      "role": "tool_developer",
      "role_display": "工具开发者",
      "created_at": "2026-08-01T10:00:00Z"
    }
  }
}
\\n
**错误响应：**

| HTTP 状态 | code | 场景 |
|---|---|---|
| 400 | 1001 | 参数校验失败 |
| 401 | 1002 | 用户名或密码错误 |
| 423 | 1003 | 账号已被禁用 |

---

## 3. MCP 网关接口

> 网关是所有 Agent 工具调用的统一入口，所有调用必须携带 JWT。

### 3.1 获取可用工具列表

**接口**：GET /mcp/tools

**权限**：需登录，仅返回调用者有权限的工具

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| category | string | 否 | 工具分类过滤，如 database、office |
| tag | string | 否 | 标签过滤，多个用逗号分隔 |
| keyword | string | 否 | 关键词模糊搜索（工具名/描述） |
| status | string | 否 | 状态过滤：published（默认）/ all |
| sort | string | 否 | 排序字段：popularity/rating/name（默认 popularity） |

**请求示例：**
\\nGET /mcp/tools?category=database&keyword=MySQL&page=1&page_size=10
\\n
### 2.2 刷新 Token

**接口**：POST /api/auth/refresh

**权限**：无需认证（需提供有效的旧 access_token）

**请求体：**

\\json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...new",
    "token_type": "Bearer",
    "expires_in": 3600
  }
}
\\n
---

### 2.3 获取当前用户信息

**接口**：GET /api/auth/me

**权限**：需登录

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "id": "usr-8f3a2b",
    "username": "agent_dev_01",
    "role": "tool_developer",
    "role_display": "工具开发者",
    "permissions": ["tool:read", "tool:call", "tool:register", "tool:view_audit"],
    "created_at": "2026-08-01T10:00:00Z",
    "last_login_at": "2026-09-08T08:30:00Z"
  }
}
\\n
---

## 1. 通用规范

### 1.1 请求头

| Header | 必填 | 说明 |
|---|---|---|
| Content-Type | 是 | application/json |
| Authorization | 受保护接口必填 | Bearer <JWT_TOKEN> |
| X-Request-ID | 否 | 客户端自定义追踪 ID，服务端会回写响应头 |

### 1.2 响应基础结构

**成功响应：**

\\json
{
  "code": 0,
  "message": "success",
  "request_id": "req-abc123xyz",
  "data": { ... }
}
\\n
**错误响应：**

\\json
{
  "code": 4001,
  "message": "Token expired",
  "request_id": "req-abc123xyz",
  "errors": [
    {
      "field": "token",
      "message": "JWT signature verification failed"
    }
  ]
}
\\n
### 1.3 通用响应字段说明

| 字段 | 类型 | 说明 |
|---|---|---|
| code | int | 业务错误码，0 表示成功 |
| message | string | 人类可读的错误/成功描述 |
| request_id | string | 本次请求唯一标识，全链路透传 |
| data | object / null | 业务数据 payload |
| errors | array | 字段级校验错误列表（可选） |

### 1.4 分页约定

所有列表接口默认支持分页，参数：

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| page | int | 1 | 页码，从 1 开始 |
| page_size | int | 20 | 每页条数，最大 100 |

响应体含分页信息：

\\json
{
  "code": 0,
  "data": {
    "items": [ ... ],
    "total": 128,
    "page": 1,
    "page_size": 20,
    "total_pages": 7
  }
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "items": [
      {
        "tool_id": "tool-mysql-query",
        "name": "mysql_query",
        "description": "执行 MySQL 查询，返回结构化结果（脱敏）",
        "category": "database",
        "tags": ["sql", "read-only"],
        "version": "v1.2.0",
        "server_id": "srv-demo-db",
        "server_name": "Demo DB Server",
        "status": "published",
        "health_status": "healthy",
        "call_count": 12840,
        "rating": 4.7,
        "input_schema": {
          "type": "object",
          "properties": {
            "sql": { "type": "string", "description": "SQL 查询语句" },
            "limit": { "type": "integer", "default": 100, "description": "最大返回行数" }
          },
          "required": ["sql"]
        },
        "is_sensitive": false,
        "sensitive_level": null
      }
    ],
    "total": 12,
    "page": 1,
    "page_size": 10
  }
}
\\n
---

### 3.2 调用工具

**接口**：POST /mcp/tools/{toolName}/call

**权限**：需登录，且对该工具具有 call 权限

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolName | string | 是 | 工具名称（URL 编码） |

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| params | object | 是 | 按工具 input_schema 提供的参数 |
| trace_id | string | 否 | 链路追踪 ID，用于关联多次调用 |

\\json
{
  "params": {
    "sql": "SELECT id, name, department FROM employees LIMIT 10",
    "limit": 10
  },
  "trace_id": "trace-abc-123"
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "message": "success",
  "request_id": "req-call-9a8b7c",
  "data": {
    "tool_name": "mysql_query",
    "server_id": "srv-demo-db",
    "result": {
      "rows": [
        { "id": 1, "name": "张三", "department": "工程部" },
        { "id": 2, "name": "李四", "department": "市场部" }
      ],
      "total_rows": 2,
      "execution_time_ms": 23
    },
    "meta": {
      "cost_estimate": 0.001,
      "latency_ms": 45,
      "server_latency_ms": 22
    }
  }
}
\\n
**错误响应：**

| HTTP 状态 | code | 场景 |
|---|---|---|
| 400 | 2001 | 参数校验失败 |
| 401 | 1001 | Token 缺失或无效 |
| 403 | 2002 | 无调用权限（敏感工具未授权） |
| 429 | 2003 | 触发限流 |
| 502 | 2004 | MCP Server 不可达或超时 |
| 500 | 2005 | 工具执行异常 |

---

## 4. 工具市场接口

### 4.1 工具分类列表

**接口**：GET /api/tools/categories

**权限**：无需认证（公开）

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "categories": [
      { "key": "database", "display": "数据库", "tool_count": 12 },
      { "key": "office", "display": "办公", "tool_count": 8 },
      { "key": "dev", "display": "研发", "tool_count": 23 },
      { "key": "marketing", "display": "营销", "tool_count": 5 }
    ]
  }
}
\\n
### 4.2 工具列表（市场端）

**接口**：GET /api/tools

**权限**：无需认证（公开浏览），登录后可查看更多字段

**查询参数**（同 /mcp/tools）：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| category | string | 否 | 分类过滤 |
| keyword | string | 否 | 关键词搜索 |
| sort | string | 否 | popularity/rating/newest |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页条数 |

### 3.3 网关健康检查

**接口**：GET /health

**权限**：无需认证

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "status": "healthy",
    "uptime_seconds": 86400,
    "services": {
      "redis": "connected",
      "postgres": "connected",
      "clickhouse": "connected"
    },
    "timestamp": "2026-09-08T12:00:00Z"
  }
}
\\n
### 4.3 工具详情

**接口**：GET /api/tools/{toolId}

**权限**：无需认证（公开浏览）

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolId | string | 是 | 工具 ID |

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "tool_id": "tool-mysql-query",
    "name": "mysql_query",
    "description": "执行 MySQL 查询，返回结构化结果（脱敏）",
    "long_description": "支持 SELECT 查询，自动脱敏敏感字段...。",
    "category": "database",
    "tags": ["sql", "read-only", "demo"],
    "version": "v1.2.0",
    "server": {
      "server_id": "srv-demo-db",
      "name": "Demo DB Server",
      "url": "http://demo-db-server:3000/mcp",
      "owner": "usr-admin-01",
      "owner_name": "平台管理员",
      "health_status": "healthy",
      "status": "published"
    },
    "input_schema": {
      "type": "object",
      "properties": {
        "sql": { "type": "string", "description": "SQL 查询语句" },
        "limit": { "type": "integer", "default": 100, "description": "最大返回行数" }
      },
      "required": ["sql"]
    },
    "output_schema": {
      "type": "object",
      "properties": {
        "rows": { "type": "array" },
        "total_rows": { "type": "integer" }
      }
    },
    "is_sensitive": false,
    "sensitive_level": null,
    "stats": {
      "total_calls": 12840,
      "success_rate": 99.2,
      "avg_latency_ms": 35,
      "rating": 4.7,
      "review_count": 23
    },
    "reviews": [
      {
        "id": "rev-001",
        "user_id": "usr-agent-02",
        "user_name": "Agent Dev 2",
        "rating": 5,
        "comment": "非常好用，查询速度快",
        "created_at": "2026-09-01T14:00:00Z"
      }
    ],
    "versions": [
      { "version": "v1.2.0", "released_at": "2026-09-01T00:00:00Z", "is_current": true },
      { "version": "v1.1.0", "released_at": "2026-08-15T00:00:00Z", "is_current": false }
    ]
  }
}
\\n
### 5.3 工具发布

**接口**：POST /api/tools/{toolId}/publish

**权限**：工具所有者或管理员

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolId | string | 是 | 工具 ID |

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| version | string | 否 | 版本号，格式 vX.Y.Z，不传则自动递增 |
| publish_note | string | 否 | 发布说明 |

\\json
{
  "version": "v1.3.0",
  "publish_note": "修复了分页参数 bug"
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "message": "工具发布成功",
  "data": {
    "tool_id": "tool-mysql-query",
    "version": "v1.3.0",
    "status": "published",
    "published_at": "2026-09-08T12:00:00Z",
    "published_by": "usr-admin-01"
  }
}
\\n
### 5.4 工具下线

**接口**：POST /api/tools/{toolId}/unpublish

**权限**：工具所有者或管理员

**请求体：**

\\json
{
  "reason": "存在安全漏洞，需要修复"
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "message": "工具已下线",
  "data": {
    "tool_id": "tool-mysql-query",
    "status": "unpublished",
    "unpublished_at": "2026-09-08T12:00:00Z"
  }
}
\\n
### 4.4 评分与评论

**接口**：POST /api/tools/{toolId}/reviews

**权限**：需登录

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolId | string | 是 | 工具 ID |

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| rating | int | 是 | 评分 1-5 |
| comment | string | 否 | 评论内容，最长 500 字 |

\\json
{
  "rating": 5,
  "comment": "查询速度很快，结果格式清晰"
}
\\n
**成功响应（201）：**

\\json
{
  "code": 0,
  "data": {
    "id": "rev-new-001",
    "tool_id": "tool-mysql-query",
    "user_id": "usr-agent-02",
    "rating": 5,
    "comment": "查询速度很快，结果格式清晰",
    "created_at": "2026-09-08T12:00:00Z"
  }
}
\\n
### 4.5 生成接入配置片段

**接口**：POST /api/tools/{toolId}/config

**权限**：需登录

**用途**：为工具市场用户提供一键生成的 MCP 接入配置代码片段（用于前端展示）

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolId | string | 是 | 工具 ID |

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| language | string | 是 | 代码语言：go / typescript |
| transport | string | 否 | 传输方式，默认 streamable-http |

\\json
{
  "language": "typescript",
  "transport": "streamable-http"
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "tool_id": "tool-mysql-query",
    "language": "typescript",
    "config_snippet": "...TypeScript 接入代码片段...",
    "env_template": "MCP_GATEWAY_URL=https://api.mcp-gateway.internal\nMCP_GATEWAY_TOKEN=YOUR_JWT_TOKEN",
    "generated_at": "2026-09-08T12:00:00Z"
  }
}
\\n
---

## 6. OpenAPI 导入接口

### 6.1 导入 OpenAPI 文档并生成 MCP 工具

**接口**：POST /api/openapi/import

**权限**：需登录（工具开发者或管理员）

**请求体（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| openapi_spec | object | 是 | OpenAPI 3.x JSON 对象 |
| server_name | string | 是 | 生成的 MCP Server 名称 |
| language | string | 否 | 目标语言：go（默认）/ typescript |
| category | string | 否 | 工具分类，如 database、office |

MVP 支持的 OpenAPI 子集：GET、POST 方法；Path/Query/Header/Body 基础参数；API Key 和 Bearer Token 认证。

不支持的 OpenAPI 特性必须返回明确错误，不得静默生成错误代码。

示例请求：

\\json
{
  "openapi_spec": {
    "openapi": "3.0.3",
    "info": { "title": "Employee API", "version": "1.0.0" },
    "servers": [{ "url": "https://hr-api.internal" }],
    "paths": {
      "/employees": {
        "get": {
          "operationId": "listEmployees",
          "summary": "获取员工列表",
          "parameters": [
            { "name": "department", "in": "query", "schema": { "type": "string" } },
            { "name": "limit", "in": "query", "schema": { "type": "integer", "default": 20 } }
          ],
          "responses": { "200": { "description": "OK" } }
        }
      }
    }
  },
  "server_name": "HR Employee Server",
  "language": "go",
  "category": "office"
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "message": "OpenAPI 导入成功，已生成 MCP 工具骨架",
  "data": {
    "import_id": "imp-x8y9z0",
    "server_name": "HR Employee Server",
    "tools_generated": [
      {
        "name": "listEmployees",
        "description": "获取员工列表",
        "input_schema": { "type": "object", "properties": { "department": { "type": "string", "description": "部门过滤" }, "limit": { "type": "integer", "default": 20 } } }
      }
    ],
    "artifacts": {
      "go_server": { "file_path": "mcp-servers/hr-employee/main.go", "download_url": "/api/openapi/downloads/imp-x8y9z0/main.go" },
      "dockerfile": { "file_path": "mcp-servers/hr-employee/Dockerfile", "download_url": "/api/openapi/downloads/imp-x8y9z0/Dockerfile" },
      "env_template": { "content": "HR_API_BASE_URL=https://hr-api.internal\nHR_API_KEY=YOUR_API_KEY" }
    },
    "warnings": []
  }
}
\\n
**错误响应：**

| code | 场景 |
|---|---|
| 6001 | OpenAPI 文档格式非法 |
| 6002 | 不支持的 OpenAPI 特性（如 PUT/DELETE、WebSocket） |
| 6003 | 缺少必需的 path 或 operationId |

---

## 5. 权限管理接口

### 5.1 查看工具权限配置

**接口**：GET /api/tools/{toolId}/permissions

**权限**：管理员或工具所有者

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolId | string | 是 | 工具 ID |

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "tool_id": "tool-mysql-query",
    "is_sensitive": false,
    "sensitive_level": null,
    "permissions": [
      { "role": "platform_admin", "can_read": true, "can_call": true },
      { "role": "tool_developer", "can_read": true, "can_call": true },
      { "role": "agent_caller", "can_read": true, "can_call": true }
    ]
  }
}
\\n
### 5.2 配置工具权限

**接口**：POST /api/tools/{toolId}/permissions

**权限**：管理员或工具所有者

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| toolId | string | 是 | 工具 ID |

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| is_sensitive | boolean | 否 | 是否标记为敏感工具 |
| sensitive_level | string | 条件必填 | 敏感等级：low/medium/high（敏感工具必填） |
| permissions | array | 是 | 各角色权限配置列表 |

\\json
{
  "is_sensitive": true,
  "sensitive_level": "high",
  "permissions": [
    { "role": "platform_admin", "can_read": true, "can_call": true },
    { "role": "tool_developer", "can_read": true, "can_call": false },
    { "role": "agent_caller", "can_read": false, "can_call": false }
  ]
}
\\n
**成功响应（200）：**

\\json
{
  "code": 0,
  "message": "权限配置已更新",
  "data": { "tool_id": "tool-mysql-query", "updated_at": "2026-09-08T12:00:00Z" }
}
\\n
---

## 9. 系统接口

### 9.1 MCP Server 注册（管理端）

**接口**：POST /api/servers

**权限**：工具开发者或管理员

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | string | 是 | Server 名称，3-128 字符 |
| url | string | 是 | MCP Server 地址（Streamable HTTP endpoint） |
| description | string | 否 | 描述 |
| contact | string | 否 | 负责人联系方式 |
| tags | array[string] | 否 | 标签列表 |

\\json
{
  "name": "Demo File Server",
  "url": "http://demo-file-server:3000/mcp",
  "description": "文件读写操作 MCP Server（演示用）",
  "contact": "dev@example.com",
  "tags": ["file", "demo"]
}
\\n
**成功响应（201）：**

\\json
{
  "code": 0,
  "message": "MCP Server 注册成功",
  "data": {
    "server_id": "srv-demo-file",
    "name": "Demo File Server",
    "url": "http://demo-file-server:3000/mcp",
    "status": "registered",
    "health_status": "checking",
    "created_at": "2026-09-08T12:00:00Z"
  }
}
\\n
### 9.2 获取 MCP Server 列表

**接口**：GET /api/servers

**权限**：无需认证（公开浏览），管理员可见更多字段

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| status | string | 否 | all/published/unpublished |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页条数 |

### 9.3 MCP Server 健康检查（手动触发）

**接口**：POST /api/servers/{serverId}/health-check

**权限**：管理员或 Server 所有者

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| serverId | string | 是 | Server ID |

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "server_id": "srv-demo-db",
    "health_status": "healthy",
    "last_check_at": "2026-09-08T12:00:00Z",
    "latency_ms": 12,
    "tool_count": 5,
    "details": {
      "connectivity": "ok",
      "mcp_version": "2024-11-05",
      "supported_transports": ["streamable-http"]
    }
  }
}
\\n
---

## 10. 统一错误格式

所有接口统一使用以下错误码体系：

### 10.1 认证/授权类

| code | HTTP 状态 | 说明 |
|---|---|---|
| 1001 | 401 | 未认证：Token 缺失、伪造或过期 |
| 1002 | 403 | 无权限：当前角色无权访问该资源 |
| 1003 | 403 | 账号已被禁用 |

### 10.2 网关/调用类

| code | HTTP 状态 | 说明 |
|---|---|---|
| 2001 | 400 | 请求参数校验失败 |
| 2002 | 403 | 工具调用权限不足（敏感工具未授权） |
| 2003 | 429 | 触发限流 |
| 2004 | 502 | MCP Server 不可达或超时 |
| 2005 | 500 | 工具执行异常 |

### 10.3 业务类

| code | HTTP 状态 | 说明 |
|---|---|---|
| 3001 | 400 | 工具不存在 |
| 3002 | 409 | 工具名已存在 |
| 3003 | 422 | 工具状态不允许此操作（未发布/已下线） |
| 4001 | 400 | OpenAPI 文档格式非法 |
| 4002 | 422 | OpenAPI 包含不支持的特性 |

### 10.4 系统类

| code | HTTP 状态 | 说明 |
|---|---|---|
| 5001 | 500 | 内部服务器错误 |
| 5002 | 503 | 服务暂时不可用（依赖服务故障） |

---

## 7. 审计日志接口

### 7.1 查询审计日志

**接口**：GET /api/audit/logs

**权限**：平台管理员

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| tool_id | string | 否 | 按工具筛选 |
| server_id | string | 否 | 按 MCP Server 筛选 |
| user_id | string | 否 | 按调用者筛选 |
| status | string | 否 | success/failed/rejected |
| start_time | string | 否 | 起始时间，ISO 8601 格式 |
| end_time | string | 否 | 结束时间，ISO 8601 格式 |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页条数 |

每次调用至少记录：request_id、调用者 ID 和角色、工具名称和 Server ID、参数摘要、调用时间、耗时、成功或失败状态、HTTP 状态码、拒绝原因、估算成本。

敏感参数不得明文写入日志，只保存脱敏结果或摘要。

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "items": [
      {
        "log_id": "log-20260908-001",
        "request_id": "req-call-9a8b7c",
        "call_time": "2026-09-08T10:30:00Z",
        "caller": { "user_id": "usr-agent-02", "username": "agent_dev_02", "role": "agent_caller" },
        "tool": { "tool_id": "tool-mysql-query", "tool_name": "mysql_query", "server_id": "srv-demo-db", "server_name": "Demo DB Server" },
        "params_summary": "{sql: SELECT ... LIMIT 10, limit: 10}",
        "params_sensitive_masked": true,
        "result": { "status": "success", "http_status": 200, "latency_ms": 45, "cost_estimate": 0.001 },
        "reject_reason": null
      }
    ],
    "total": 5842,
    "page": 1,
    "page_size": 20
  }
}
\\n
---

## 8. 统计分析接口

### 8.1 统计概览

**接口**：GET /api/analytics/overview

**权限**：平台管理员

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| days | int | 否 | 统计近 N 天数据，默认 7 |

统计页面至少展示：总调用次数、工具调用趋势、Top 工具排行、成功率和失败率、拒绝调用次数、敏感工具异常高频调用告警。

**成功响应（200）：**

\\json
{
  "code": 0,
  "data": {
    "period": { "start": "2026-09-01T00:00:00Z", "end": "2026-09-08T00:00:00Z", "days": 7 },
    "summary": {
      "total_calls": 58420,
      "successful_calls": 57890,
      "failed_calls": 312,
      "rejected_calls": 218,
      "success_rate": 99.09,
      "avg_latency_ms": 38.5,
      "p99_latency_ms": 4.2,
      "total_cost_estimate": 58.42
    },
    "top_tools": [
      { "tool_id": "tool-mysql-query", "tool_name": "mysql_query", "call_count": 12840, "success_rate": 99.5 },
      { "tool_id": "tool-read-file", "tool_name": "read_file", "call_count": 8320, "success_rate": 98.8 }
    ],
    "daily_trend": [
      { "date": "2026-09-02", "calls": 7800, "success": 7750, "rejected": 32 },
      { "date": "2026-09-03", "calls": 8200, "success": 8150, "rejected": 28 }
    ],
    "alerts": [
      {
        "alert_id": "alert-001",
        "type": "sensitive_tool_high_frequency",
        "severity": "high",
        "tool_id": "tool-sensitive-salary",
        "tool_name": "get_salary",
        "message": "敏感工具 get_salary 在 1 小时内被调用 120 次，超过阈值 50",
        "triggered_at": "2026-09-07T15:30:00Z"
      }
    ]
  }
}
\\n
### 8.2 调用趋势明细

**接口**：GET /api/analytics/trends

**权限**：平台管理员

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| start_time | string | 是 | 起始时间 |
| end_time | string | 是 | 结束时间 |
| granularity | string | 否 | 粒度：hour/day（默认 hour） |
| tool_id | string | 否 | 指定工具 ID 过滤 |

---

## 11. 角色与权限矩阵

| 操作 | 平台管理员 | 工具开发者 | Agent 调用者 |
|---|:---:|:---:|:---:|
| 浏览工具列表 | Yes | Yes | Yes |
| 查看工具详情 | Yes | Yes | Yes |
| 调用工具 | Yes | Yes | Yes（仅已授权） |
| 注册 MCP Server | Yes | Yes | No |
| 编辑工具信息 | Yes | Yes（仅自有） | No |
| 发布/下线工具 | Yes | Yes（仅自有） | No |
| 配置工具权限 | Yes | No | No |
| 导入 OpenAPI | Yes | Yes | No |
| 查看审计日志 | Yes | No | No |
| 查看统计概览 | Yes | Yes（仅自有工具） | No |
| 评分/评论 | Yes | Yes | Yes |
| 用户管理 | Yes | No | No |

---

## 附录 A：MVP 不包含的接口

以下功能不在 MVP 范围内，接口暂不实现：

- POST /api/users — 用户注册（Demo 环境内置账户）
- PUT /api/users/{id} — 用户信息修改
- DELETE /api/users/{id} — 用户删除
- /api/billing/* — 计费结算
- POST /api/workflows/* — 工作流编排
- MCP SSE / stdio 传输方式相关接口

---

## 12. 限流策略

使用 Redis 令牌桶算法实现，按用户+工具维度限流。

### 12.1 默认限流阈值

| 维度 | 限制 | 说明 |
|---|---|---|
| 全局（按用户） | 60 req/min | 每个用户的总调用频率 |
| 全局（按角色） | 200 req/min | 每角色的聚合频率 |
| 单工具 | 30 req/min | 单个工具的单用户调用频率 |
| 敏感工具 | 5 req/min | 高敏感工具的额外限流 |

### 12.2 限流响应

超过限制时返回：

\\json
{
  "code": 2003,
  "message": "Rate limit exceeded. Please retry after 30s.",
  "request_id": "req-rate-limited-001",
  "data": {
    "retry_after_seconds": 30,
    "limit": {
      "type": "user",
      "max_requests": 60,
      "window_seconds": 60
    }
  }
}
\\n
响应头携带限流信息：

`
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 3
X-RateLimit-Reset: 1725800400
Retry-After: 30
`
