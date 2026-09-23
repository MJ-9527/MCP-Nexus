$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

Set-Location $root
docker compose -f deploy/docker-compose.yml up -d --build

$deadline = (Get-Date).AddMinutes(3)
do {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:15173" -UseBasicParsing -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            break
        }
    } catch {
        Start-Sleep -Seconds 2
    }
} while ((Get-Date) -lt $deadline)

if ((Get-Date) -ge $deadline) {
    throw "MCP-Nexus 启动超时，请运行 docker compose -f deploy/docker-compose.yml ps 检查服务状态。"
}

Start-Process "http://localhost:15173"
Write-Host "MCP-Nexus 已启动：http://localhost:15173"
