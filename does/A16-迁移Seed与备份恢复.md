# A16 迁移、Seed 与备份恢复

## 初始化新环境

```powershell
docker compose --env-file .env -f deploy/docker-compose.yml up -d
go run ./cmd/db-migrate -seed db/seed/demo.sql
```

迁移工具创建 `schema_migrations`，按文件名顺序执行尚未应用的 SQL；每个文件在单独事务中执行。Seed 可重复运行。

演示账号仅用于本地联调：

| 用户 | 密码 | 角色 |
|---|---|---|
| `demo_admin` | `DemoAdmin123!` | `platform_admin` |
| `demo_developer` | `DemoDeveloper123!` | `tool_developer` |
| `demo_agent` | `DemoAgent123!` | `agent_caller` |

生产环境不得使用演示密码或执行演示 Seed。

## 备份

```powershell
./scripts/backup.ps1
```

默认生成 `backups/mcp_platform.dump`，使用 PostgreSQL custom format。

## 恢复验证

```powershell
./scripts/restore.ps1 -BackupPath backups/mcp_platform.dump -Database mcp_platform_restore
docker exec mcp-nexus-postgres psql -U mcp_user -d mcp_platform_restore -c "SELECT count(*) FROM mcp_tools;"
```

恢复脚本只重建明确指定的数据库，默认使用 `mcp_platform_restore`，不会覆盖开发库。

## 空库验收

推荐创建独立临时数据库，修改当前进程的 `POSTGRES_DB` 后执行迁移和 Seed。验证至少包含：迁移数量、三个角色、三个用户、Server、Tool、版本、权限、评分、审计、告警和适配任务。
