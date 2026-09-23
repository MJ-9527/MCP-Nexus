# MCP-Nexus API 接口文档（当前代码版）

> 本文根据当前 `router/router.go`、`handler/` 和 `model/` 实现整理。代码变更后应同步更新本文。

## 1. 通用约定

- 默认地址：`http://localhost:8080`
- JSON 请求和响应使用 `Content-Type: application/json`。
- 除 `/health` 和 `/api/auth/login` 外，`/api` 管理接口需要：

```http
Authorization: Bearer <JWT>
```

- Server、Tool、权限、审计、导入、分析等管理写操作还需要 `platform_admin` 角色。
- `/mcp/*` 需要 JWT，并使用 Redis 限流：`platform_admin=600/min`、`tool_developer=300/min`、`agent_caller=120/min`。
- 每个请求响应通常使用统一包装：

```json
{
  "code": "OK",
  "message": "ok",
  "request_id": "uuid",
  "data": {}
}
```

错误响应仍使用同一包装，`code` 会是业务错误码，例如 `INVALID_PARAMETER`、`FORBIDDEN`、`NOT_FOUND`。

## 2. 健康检查与认证

### GET `/health`

无需认证。返回服务存活状态。

```json
{"code":"OK","message":"ok","request_id":"...","data":{"status":"ok"}}
```

### POST `/api/auth/login`

无需认证，登录后返回 JWT。

请求：

```json
{"username":"demo_admin","password":"DemoAdmin123!"}
```

字段：`username`、`password` 必填，长度最多 100。

成功：`200`。账号或密码错误：`401`；账号禁用：`403`。

## 3. Server 注册中心

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/servers` | JWT | Server 列表 |
| GET | `/api/servers/:id` | JWT | 查询 Server |
| POST | `/api/servers` | 管理员 | 注册 Server |
| POST | `/api/servers/:id/activate` | 管理员 | 上线 |
| POST | `/api/servers/:id/offline` | 管理员 | 下线 |
| POST | `/api/servers/:id/health-check` | 管理员 | 执行健康检查 |
| POST | `/api/servers/:id/import-openapi` | 管理员 | 导入 OpenAPI |
| POST | `/api/servers/:id/import-skills` | 管理员 | 导入 Skills |

注册请求：

```json
{
  "name":"demo-server",
  "description":"示例 MCP 服务",
  "endpoint":"http://localhost:9001",
  "version":"1.0.0",
  "owner_id":1
}
```

`name`、`endpoint`、`version` 必填；`endpoint` 必须是 URL。

状态值主要为 `active`、`offline`；查询结果包含 `id/name/description/endpoint/version/owner_id/status/health_status/last_health_check_at/created_at/updated_at`。

OpenAPI 导入请求：

```json
{"spec":{},"published":false}
```

Skills 导入请求：

```json
{
  "published":false,
  "tools":[{
    "name":"search",
    "description":"搜索",
    "input_schema":{"type":"object"},
    "version":"1.0.0",
    "endpoint":"",
    "method":"POST",
    "path_template":"/tools/search/call",
    "params":{"path":[],"query":[],"body":""}
  }]
}
```

## 4. Tool 市场

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/tools` | JWT | 工具列表 |
| GET | `/api/tools/:id` | JWT | 工具详情 |
| POST | `/api/tools` | 管理员 | 注册工具 |
| POST | `/api/tools/:id/publish` | 管理员 | 发布工具 |
| POST | `/api/tools/:id/offline` | 管理员 | 下线工具 |

列表查询参数：`q`、`keyword`、`category`、`tags`/`tag`、`status`、`published`、`server_id`、`health_status`、`sort`、`page`、`page_size`。

注册请求：

```json
{
  "server_id":1,
  "name":"search",
  "description":"搜索工具",
  "category":"search",
  "tags":["web","query"],
  "input_schema":{"type":"object","properties":{}},
  "version":"1.0.0"
}
```

工具对象包含 `id/server_id/name/description/category/tags/input_schema/version/published/is_sensitive/sensitive_level/health_status/call_count/average_rating/rating_count/created_at/updated_at`。

## 5. 权限、评价与适配任务

### Tool 权限

| 方法 | 路径 | 权限 |
|---|---|---|
| GET | `/api/tools/:id/permissions` | 管理员 |
| POST | `/api/tools/:id/permissions` | 管理员 |
| DELETE | `/api/tools/:id/permissions` | 管理员 |
| GET | `/api/tools/:id/permissions/check` | 管理员 |

批量配置请求：

```json
{
  "is_sensitive":true,
  "sensitive_level":"high",
  "permissions":[
    {"role":"platform_admin","can_read":true,"can_call":true},
    {"role":"tool_developer","can_read":true,"can_call":false}
  ]
}
```

`sensitive_level` 只能是 `low`、`medium`、`high`。接口也兼容旧单条格式：`user_id`、`role_id`、`action`，其中 `action` 为 `read` 或 `call`。

查询结果：`tool_id/is_sensitive/sensitive_level/permissions[]`，权限项为 `role/can_read/can_call`。

权限检查参数：`user_id`、`action=read|call`。无权限返回 `403`。

### 评价

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tools/:id/reviews` | 分页查询评价 |
| POST | `/api/tools/:id/reviews` | 新增评价 |

新增评价请求：`{"rating":5,"comment":"很好用"}`，评分范围为 1–5。查询支持 `page`、`page_size`。

### 适配任务

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tools/:id/adaptation-tasks` | 查询工具任务 |
| POST | `/api/tools/:id/adaptation-tasks` | 创建任务 |
| GET | `/api/tools/adaptation-tasks/:task_id` | 查询任务 |
| PATCH | `/api/tools/adaptation-tasks/:task_id` | 更新状态 |

创建请求：`{"task_type":"openapi","source_url":"https://example.com/openapi.json"}`。

更新请求：`{"status":"running","error_message":""}`。

## 6. 审计日志与分析

### 审计日志

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/audit/logs` | 写入审计日志 |
| GET | `/api/audit/logs` | 分页查询 |
| POST | `/api/audit-logs` | 旧路径兼容 |
| GET | `/api/audit-logs` | 旧路径兼容 |

写入请求：

```json
{
  "request_id":"req-001",
  "user_id":2,
  "tool_id":1,
  "server_id":1,
  "tool_name":"search",
  "caller_role":"agent_caller",
  "duration_ms":120,
  "status":"success",
  "http_status":200,
  "denied_reason":"",
  "reject_reason":"",
  "cost_estimate":0.01,
  "parameters":{"query":"hello","token":"[masked]"}
}
```

查询参数：`request_id`、`tool_id`、`server_id`、`user_id`、`status`、`start_time`、`end_time`、`page`、`page_size`。默认 `page=1`、`page_size=20`，最大 `100`。

审计日志不会保存原始参数；返回 `params_summary`、`params_digest`、`params_sensitive_masked`。

### 分析接口

| 方法 | 路径 | 参数 |
|---|---|---|
| GET | `/api/analytics/overview` | `days`，默认 7，范围 1–365 |
| GET | `/api/analytics/trends` | `start_time`、`end_time`（RFC3339）、`granularity`、`tool_id` |
| GET | `/api/analytics/alerts` | `status`、`severity`、`page`、`page_size` |
| POST | `/api/analytics/alerts/:id/acknowledge` | 无请求体 |
| GET | `/api/metrics` | `window`，查询运行指标 |

## 7. MCP 调用代理

### GET `/mcp/tools`

JWT 必填。返回当前用户可访问的工具：

```json
{"tools":[{"name":"search","description":"搜索","input_schema":{"type":"object"}}]}
```

### POST `/mcp/tools/:toolName/call`

JWT 必填，受 Redis 角色限流和 Tool 权限检查。

请求：

```json
{
  "method":"tools/call",
  "arguments":{"query":"hello"}
}
```

路径中的 `toolName` 会覆盖请求体中的同名字段。成功时透传下游 MCP 响应；常见错误：`403 FORBIDDEN`、`404 TOOL_NOT_FOUND`、`409 TOOL_OFFLINE`、`502 UPSTREAM_ERROR`、`504 UPSTREAM_TIMEOUT`。

### GET `/api/mcp-config`

JWT 必填。返回当前账号的 MCP 接入地址、Bearer Token 说明、操作示例和可用工具列表。

## 8. 错误状态码

| HTTP | 含义 |
|---:|---|
| 400 | 请求参数、JSON、时间或分页参数错误 |
| 401 | 缺少或无效 JWT、登录失败 |
| 403 | 角色不足或 Tool 无权限 |
| 404 | Server、Tool、任务或告警不存在 |
| 409 | 状态冲突、Tool 下线等业务冲突 |
| 429 | Redis 限流 |
| 502 | 下游服务返回错误 |
| 503 | 下游 Server 不可用 |
| 504 | 下游调用超时 |
| 500 | 服务内部错误 |

## 9. 运行时配置

主要环境变量：`PORT`、`POSTGRES_HOST`、`POSTGRES_PORT`、`POSTGRES_DB`、`POSTGRES_USER`、`POSTGRES_PASSWORD`、`JWT_SECRET`、`JWT_TTL`、`REDIS_ADDR`、`PUBLIC_BASE_URL`、`CLICKHOUSE_ADDR`、`CLICKHOUSE_DATABASE`、`CLICKHOUSE_USER`、`CLICKHOUSE_PASSWORD`。

