# Skills 示例包与 MCP 适配配置

本目录提供一个可运行的 **Skills 工具包** 示例，以及把它暴露为 MCP Server 的适配器。

## 目录结构

```text
examples/skills/
├── skills.json          # Skills 声明：工具名、描述、输入/输出 Schema
├── adapter/
│   ├── main.go          # 适配器：读取 skills.json，暴露 /health 和 /tools/:name/call
│   ├── go.mod           # 独立 module，纯标准库
│   └── Dockerfile       # 容器化
└── README.md            # 本文件
```

## Skills 工具列表


| 工具名          | 描述                | 输入                              | 输出                     |
| --------------- | ------------------- | --------------------------------- | ------------------------ |
| `calculate_vat` | 计算含税总价        | `amount: number`, `rate?: number` | `amount`, `tax`, `total` |
| `format_phone`  | 手机号 3-4-4 格式化 | `phone: string`                   | `phone`, `formatted`     |

## 本地运行

```bash
cd examples/skills/adapter
go run .
```

服务默认监听 `:8082`。

## 调用示例

```bash
# 健康检查
curl http://localhost:8082/health

# 查看 Skills 清单
curl http://localhost:8082/.well-known/skills.json

# 调用 calculate_vat
curl -X POST http://localhost:8082/tools/calculate_vat/call \
  -H 'Content-Type: application/json' \
  -d '{"amount": 100, "rate": 0.13}'

# 调用 format_phone
curl -X POST http://localhost:8082/tools/format_phone/call \
  -H 'Content-Type: application/json' \
  -d '{"phone": "13812345678"}'
```

## 注册到 MCP 网关

1. 把适配器加入 `deploy/docker-compose.yml`（参考 demo-service）。
2. 在数据库 `mcp_servers` 中注册：
   ```sql
   INSERT INTO mcp_servers (name, description, endpoint, version, status, health_status)
   VALUES ('demo-skills', 'Skills 示例包', 'http://demo-skills:8082', '1.0.0', 'active', 'unknown');
   ```
3. 把 `skills.json` 中的工具同步到 `mcp_tools` 表（或等成员 A 实现自动同步）。
4. 通过网关调用：
   ```bash
   POST /mcp/tools/calculate_vat/call
   ```

## 完成标准

- [X]  提供可运行的 Skills 示例包
- [X]  定义工具描述、输入/输出 Schema
- [X]  提供 MCP 适配配置和启动说明
- [X]  可通过 `/tools/:name/call` 被网关调用
