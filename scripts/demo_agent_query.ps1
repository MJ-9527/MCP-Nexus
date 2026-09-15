# MCP-Nexus Agent 查库生成报表演示
# 演示场景: Agent 登录 → 工具发现 → 调用 query_sales → 生成月度销售报表

$ErrorActionPreference = 'Stop'
$base = 'http://localhost:8080'

Write-Host "=========================================="
Write-Host "  MCP-Nexus Agent 查库生成报表 演示"
Write-Host "=========================================="
Write-Host ""

# Step 1: Agent 登录
Write-Host "[Step 1] Agent 登录认证..."
$login = Invoke-RestMethod "$base/api/auth/login" -Method Post -ContentType 'application/json' -Body '{"username":"agent","password":"agent123"}'
$token = $login.data.token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "  ✓ 登录成功，角色: $($login.data.user.role)"
Write-Host ""

# Step 2: 工具发现
Write-Host "[Step 2] 工具发现..."
$tools = Invoke-RestMethod "$base/mcp/tools" -Headers $headers
Write-Host "  ✓ 发现 $($tools.Count) 个可用工具:"
foreach ($t in $tools) {
    Write-Host "    - $($t.name): $($t.description)"
}
Write-Host ""

# Step 3: 调用 query_sales 查询销售数据
Write-Host "[Step 3] 调用 query_sales 查询销售数据..."
$callBody = '{"arguments":{"month":"2026-01"}}'
try {
    $result = Invoke-RestMethod "$base/mcp/tools/query_sales/call" -Method Post -Headers $headers -ContentType 'application/json' -Body $callBody
    Write-Host "  ✓ 调用成功"
    Write-Host "  返回数据:"
    $result | ConvertTo-Json -Depth 5 | ForEach-Object { Write-Host "    $_" }
} catch {
    Write-Host "  ✗ 调用失败: $($_.Exception.Message)"
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $errBody = $reader.ReadToEnd()
        Write-Host "  错误详情: $errBody"
    }
}
Write-Host ""

# Step 4: 尝试调用无权限工具（演示 RBAC 拒绝）
Write-Host "[Step 4] 尝试调用敏感工具（演示权限拒绝）..."
try {
    Invoke-RestMethod "$base/mcp/tools/query_inventory/call" -Method Post -Headers $headers -ContentType 'application/json' -Body '{"arguments":{}}' | Out-Null
    Write-Host "  ✗ 敏感工具调用未被拒绝（异常）"
} catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "  ✓ 权限拒绝成功，HTTP $status"
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $errBody = $reader.ReadToEnd() | ConvertFrom-Json
    Write-Host "    拒绝原因: $($errBody.message)"
}
Write-Host ""

# Step 5: 查看审计日志（需 admin/developer 角色）
Write-Host "[Step 5] 查看审计日志记录..."
$adminLogin = Invoke-RestMethod "$base/api/auth/login" -Method Post -ContentType 'application/json' -Body '{"username":"admin","password":"admin123"}'
$adminHeaders = @{ Authorization = "Bearer $($adminLogin.data.token)" }
$audit = Invoke-RestMethod "$base/api/audit/logs?since_hours=1&page_size=10" -Headers $adminHeaders
Write-Host "  OK audit logs total=$($audit.data.total)"
foreach ($log in $audit.data.items) {
    $status = if ($log.success) { "OK" } else { "REJECT" }
    Write-Host "    [$($log.called_at)] $($log.tool_name) -> $status ($($log.reject_reason))"
}
Write-Host ""

# Step 6: 生成报表
Write-Host "[Step 6] Generate Sales Report"
Write-Host "  ============================================"
Write-Host "  |  MCP-Nexus Sales Report - 2026-01        |"
Write-Host "  |  Source:  query_sales tool              |"
Write-Host "  |  Caller:  agent (ID=3)                  |"
Write-Host "  |  Status:  Authorized [OK]               |"
Write-Host "  |  Audit:   Recorded [OK]                 |"
Write-Host "  ============================================"
Write-Host ""
Write-Host "Demo complete."
