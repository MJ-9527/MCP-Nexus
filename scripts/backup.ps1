param([string]$Output = "backups/mcp_platform.dump",[string]$Database = "mcp_platform")
$ErrorActionPreference = "Stop"
$absolute = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\$Output"))
$directory = Split-Path -Parent $absolute
New-Item -ItemType Directory -Force -Path $directory | Out-Null
$containerFile = "/tmp/mcp-platform-backup.dump"
if ($Database -notmatch '^[a-zA-Z_][a-zA-Z0-9_]*$') { throw "Invalid database name" }
docker exec mcp-nexus-postgres pg_dump -U mcp_user -d $Database -Fc -f $containerFile
if ($LASTEXITCODE -ne 0) { throw "pg_dump failed" }
docker cp "mcp-nexus-postgres:$containerFile" $absolute
if ($LASTEXITCODE -ne 0) { throw "docker cp failed" }
docker exec mcp-nexus-postgres rm -f $containerFile
Write-Host "Backup created: $absolute"
