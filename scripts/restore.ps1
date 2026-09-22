param([Parameter(Mandatory=$true)][string]$BackupPath,[string]$Database = "mcp_platform_restore")
$ErrorActionPreference = "Stop"
if ($Database -notmatch '^[a-zA-Z_][a-zA-Z0-9_]*$') { throw "Invalid database name" }
$absolute = [System.IO.Path]::GetFullPath($BackupPath)
if (-not (Test-Path -LiteralPath $absolute)) { throw "Backup not found: $absolute" }
$containerFile = "/tmp/mcp-platform-restore.dump"
docker cp $absolute "mcp-nexus-postgres:$containerFile"
docker exec mcp-nexus-postgres dropdb -U mcp_user --if-exists $Database
docker exec mcp-nexus-postgres createdb -U mcp_user $Database
docker exec mcp-nexus-postgres pg_restore -U mcp_user -d $Database --clean --if-exists $containerFile
if ($LASTEXITCODE -ne 0) { throw "pg_restore failed" }
docker exec mcp-nexus-postgres rm -f $containerFile
Write-Host "Restored database: $Database"
