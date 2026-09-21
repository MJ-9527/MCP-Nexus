# A13 ClickHouse 正式部署与 GoLand 连接

项目统一通过 `deploy/docker-compose.yml` 创建 ClickHouse，不再依赖手工创建的容器、临时用户或放宽 `default` 用户权限。

## 账户原则

- `default` 用户保留 ClickHouse 默认的本机访问限制。
- 应用和 IDE 使用专用用户 `mcp_nexus`。
- 数据库为 `mcp_nexus`。
- 密码必须通过根目录 `.env` 的 `CLICKHOUSE_PASSWORD` 提供，不提交真实密码。

## GoLand 配置

- Driver：最新版官方 ClickHouse JDBC。
- JDBC URL：`jdbc:clickhouse:http://localhost:8123/mcp_nexus`。
- User：`mcp_nexus`。
- Password：与 `.env` 的 `CLICKHOUSE_PASSWORD` 相同。
- Advanced / Driver properties 中不得存在 `databaseTerm` 和 `session_id`。这两个属性不是当前 ClickHouse JDBC 支持的连接参数。

项目 `.idea/dataSources.xml` 已固定 JDBC URL，`.idea/dataSources.local.xml` 已固定用户名。密码仍由 GoLand 密钥存储保存，不应写入仓库。

## 端口

- `8123`：HTTP/JDBC 和 Go 应用连接。
- `9000`：ClickHouse Native 协议，供需要原生协议的客户端使用。
