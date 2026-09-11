# 角色 C：基础设施、示例服务与健康检查

## 负责范围

负责 Docker Compose、示例 MCP Server、健康检查、演示数据、Redis/ClickHouse 环境、OpenAPI/Skills 示例和可观测性基础设施。

## 主要目录

`deploy/`、`examples/`、`scripts/`、`adapter/`、`db/`

## 第 1 周

### C1：完善 Docker Compose 基础环境

- 配置 PostgreSQL、网关和示例服务的容器、网络、端口和卷。
- 增加环境变量模板、健康检查和依赖启动说明。
- 完成标准：`docker compose up -d` 可启动基础服务。

### C2：实现示例 MCP Server

- 实现 `GET /health` 和 `POST /tools/query_sales/call`。
- 返回固定的脱敏销售数据和统一错误格式。
- 完成标准：示例服务可独立启动并响应健康与工具请求。

### C3：准备 Seed 与演示数据

- 提供 demo Server、`query_sales` Tool、版本和状态初始化数据。
- 确保数据可重复执行且不产生冲突。
- 完成标准：新数据库可加载最小联调数据。

### C4：实现健康检查运行支持

- 实现访问 `{endpoint}/health` 的检查 Client 或任务支持。
- 配合保存 online/offline、检查时间和延迟信息。
- 完成标准：手动健康检查可更新 Server 状态。

## 第 2 周

### C5：增加数据库类示例工具

- 提供一个查询脱敏业务数据的数据库工具示例。
- 明确输入 Schema、返回格式和错误场景。
- 完成标准：数据库工具可通过网关成功调用。

### C6：增加文件与 HTTP 示例工具

- 提供文件读取和外部 HTTP 请求两个示例工具。
- 对路径、域名、响应大小和敏感字段进行安全限制。
- 完成标准：两类工具至少各有一个成功和一个失败场景。

### C7：增加敏感工具与服务鉴权

- 增加 `delete_customer` 示例，用于验证无权限拒绝。
- 配置示例服务鉴权 Header、错误状态和超时行为。
- 完成标准：敏感工具在无权限时不会执行下游操作。

### C8：接入 Redis 与 ClickHouse 容器

- 将 Redis、ClickHouse 加入 Compose，配置卷、端口和健康检查。
- 编写连接参数和故障排查说明。
- 完成标准：B 可连接 Redis 限流，A/B 可写入或查询 ClickHouse。

## 第 3 周

### C9：准备 OpenAPI 示例文档

- 提供覆盖 GET、POST、Query、Header、Body 的 OpenAPI 3.x 示例。
- 包含 API Key、Bearer Token 和基础响应 Schema。
- 完成标准：示例可被导入任务正确解析。

### C10：实现 OpenAPI → MCP 生成模板

- 实现限定子集的 Go 或 TypeScript MCP Server 骨架生成。
- 生成工具名、描述、输入 Schema、环境变量模板和 Dockerfile。
- 完成标准：生成产物可编译、可启动并调用示例 API。

### C11：实现不支持特性的错误处理

- 对不支持的 HTTP 方法、Schema 或鉴权方式返回明确错误。
- 增加生成器最小测试和失败示例。
- 完成标准：不会静默生成错误代码。

### C12：准备 Skills 示例与适配配置

- 提供一个可运行 Skills 示例包和 MCP 适配配置。
- 定义工具描述、输入输出 Schema 和启动说明。
- 完成标准：Skills 示例可注册并通过网关调用。

## 第 4 周

### C13：完善一键部署与依赖等待

- 增加重启策略、服务依赖、健康检查和启动等待逻辑。
- 验证空环境下 PostgreSQL、Redis、ClickHouse、示例服务和网关启动。
- 完成标准：按 README 可一键启动核心环境。

### C14：补充日志、指标和链路基础设施

- 使用结构化日志记录服务、请求和 `request_id`。
- 提供基础指标和 OpenTelemetry 接入配置或说明。
- 完成标准：服务异常时可以定位组件和请求。

### C15：执行备份恢复与故障演练

- 演练 PostgreSQL/ClickHouse 备份恢复、服务重启和依赖不可用场景。
- 记录命令、预期结果和已知限制。
- 完成标准：恢复步骤可由其他成员复现。

### C16：准备答辩演示环境

- 固化演示数据、端口、启动脚本和成功/拒绝/故障演示脚本。
- 汇总端口冲突、环境变量和数据库连接排查方法。
- 完成标准：演示环境可稳定重复运行完整链路。

## 角色最终验收标准

- 核心依赖可通过 Docker Compose 启动并通过健康检查。
- 示例 MCP Server、OpenAPI 生成器和 Skills 示例均有可复现运行方式。
- 空环境、故障环境和恢复环境都有验证记录。

## 依赖与交接

- 依赖 A 确定数据库模型、Seed 字段和健康状态更新接口。
- 为 B 提供可调用的下游服务、Redis/ClickHouse 地址和适配器产物。
- 为 D 提供 Compose 启动命令、演示数据和故障排查文档。
