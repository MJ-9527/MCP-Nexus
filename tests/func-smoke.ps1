# MCP-Nexus 全量接口功能冒烟测试（当前版本鉴权：JWT + RBAC）
# 前置：PostgreSQL/Redis/ClickHouse 已运行；已执行 db 迁移与 db/seed.sql；
#       demo-service 监听 8081；网关监听 $BaseURL（默认 18080）。
# 用法：pwsh ./tests/func-smoke.ps1 [-BaseURL http://localhost:18080]
param([string]$BaseURL = "http://localhost:18080")
$ErrorActionPreference = "Continue"
$script:pass = 0; $script:fail = 0; $script:failures = @()

function Check($name, $cond, $detail) {
    if ($cond) { Write-Host "  [PASS] $name" -ForegroundColor Green; $script:pass++ }
    else { Write-Host "  [FAIL] $name $detail" -ForegroundColor Red; $script:fail++; $script:failures += $name }
}
function Req($method, $path, $body, $token, $extraHeaders) {
    $h = @{ 'Content-Type' = 'application/json' }
    if ($token) { $h['Authorization'] = "Bearer $token" }
    if ($extraHeaders) { foreach ($k in $extraHeaders.Keys) { $h[$k] = $extraHeaders[$k] } }
    $a = @{ Uri = "$BaseURL$path"; Method = $method; Headers = $h; TimeoutSec = 15; UseBasicParsing = $true }
    if ($null -ne $body) { $a.Body = ($body | ConvertTo-Json -Depth 8 -Compress) }
    try {
        return Invoke-WebRequest @a
    } catch {
        $resp = $_.Exception.Response
        if ($null -eq $resp) { throw }
        $stream = $resp.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($stream)
        $content = $reader.ReadToEnd()
        $reader.Close()
        return [pscustomobject]@{ StatusCode = [int]$resp.StatusCode; Content = $content; Headers = $resp.Headers }
    }
}
function Login($u, $p) {
    $r = Req 'POST' '/api/auth/login' @{ username = $u; password = $p } $null
    return ($r.Content | ConvertFrom-Json).data.token
}

Write-Host "`n=== MCP-Nexus 功能冒烟 ($BaseURL) ===" -ForegroundColor Cyan

# 0. 健康检查
$r = Req 'GET' '/health' $null $null
Check 'GET /health 200' ($r.StatusCode -eq 200) $r.Content

# 1. 认证
$r = Req 'POST' '/api/auth/login' @{} $null
Check '登录缺参数 400' ($r.StatusCode -eq 400) $r.Content
$r = Req 'POST' '/api/auth/login' @{ username = 'admin'; password = 'wrong-pass' } $null
Check '错误密码 401' ($r.StatusCode -eq 401) $r.Content

$admin = Login 'admin' 'password123'
Check 'admin 登录拿到 token' ([bool]$admin)
$dev = Login 'dev01' 'password123'
Check 'dev01 登录拿到 token' ([bool]$dev)
$agent = Login 'agent01' 'password123'
Check 'agent01 登录拿到 token' ([bool]$agent)
$restricted = Login 'restricted' 'password123'
Check 'restricted 登录拿到 token' ([bool]$restricted)

$r = Req 'GET' '/api/auth/me' $null $admin
$me = ($r.Content | ConvertFrom-Json).data
Check 'GET /api/auth/me 200' ($r.StatusCode -eq 200) $r.Content
Check 'me 角色为 platform_admin' ($me.role -eq 'platform_admin') "role=$($me.role)"

$r = Req 'GET' '/api/servers' $null $null
Check '未带 token 访问受保护接口 401' ($r.StatusCode -eq 401) $r.Content

# 2. Server 管理
$r = Req 'GET' '/api/servers' $null $admin
Check 'admin 列出 Servers 200' ($r.StatusCode -eq 200) $r.Content

$srvName = "smoke-server-$(Get-Random -Maximum 90000 + 10000)"
$r = Req 'POST' '/api/servers' @{ name = $srvName; endpoint = 'http://localhost:8081'; version = '1.0.0'; description = 'smoke' } $admin
Check 'admin 注册 Server 200' ($r.StatusCode -eq 200) $r.Content
$newSid = ($r.Content | ConvertFrom-Json).data.id
$r = Req 'POST' '/api/servers' @{ name = $srvName; endpoint = 'http://localhost:8081'; version = '1.0.0' } $admin
Check '重名注册 409' ($r.StatusCode -eq 409) $r.Content
$r = Req 'POST' '/api/servers' @{ name = 'bad-url'; endpoint = 'xxx'; version = '1.0.0' } $admin
Check '非法 endpoint 400' ($r.StatusCode -eq 400) $r.Content
$r = Req 'POST' '/api/servers' @{ name = 'agent-srv'; endpoint = 'http://localhost:8081'; version = '1.0.0' } $agent
Check 'agent_caller 注册 Server 403' ($r.StatusCode -eq 403) $r.Content
$r = Req 'GET' "/api/servers/$newSid" $null $admin
Check '按 ID 查询 Server 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'GET' '/api/servers/99999999' $null $admin
Check '查询不存在 Server 404' ($r.StatusCode -eq 404) $r.Content

# demo-db-server(id=1) 激活 + 健康检查（探测真实 demo-service）
$r = Req 'POST' '/api/servers/1/activate' $null $admin
Check '激活 demo-db-server 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'POST' '/api/servers/1/health-check' $null $admin
Check '健康检查 demo-db-server 200' ($r.StatusCode -eq 200) $r.Content

# 下线新注册 server 验证写操作
$r = Req 'POST' "/api/servers/$newSid/offline" $null $admin
Check '下线 Server 200' ($r.StatusCode -eq 200) $r.Content

# 3. Tool 市场与管理
$r = Req 'GET' '/api/tools?page=1&page_size=10' $null $admin
$toolsData = ($r.Content | ConvertFrom-Json).data
Check '工具市场列表 200' ($r.StatusCode -eq 200) $r.Content
Check '列表含 items 且数量>0' ($toolsData.items.Count -gt 0) "count=$($toolsData.items.Count)"
$salesId = ($toolsData.items | Where-Object { $_.name -eq 'query_sales' }).id
Check '能找到 query_sales' ([bool]$salesId)

$r = Req 'GET' "/api/tools/$salesId" $null $agent
Check 'agent 查看工具详情 200' ($r.StatusCode -eq 200) $r.Content

$toolName = "smoke_tool_$(Get-Random -Maximum 90000 + 10000)"
$r = Req 'POST' '/api/tools' @{ server_id = 1; name = $toolName; category = 'database'; version = '1.0.0'; input_schema = '{"type":"object"}'; description = 'smoke tool' } $admin
Check 'admin 注册 Tool 200' ($r.StatusCode -eq 200) $r.Content
$newTid = ($r.Content | ConvertFrom-Json).data.id
$r = Req 'POST' "/api/tools/$newTid/publish" $null $admin
Check '发布 Tool 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'POST' "/api/tools/$newTid/offline" $null $agent
Check 'agent 下线 Tool 403' ($r.StatusCode -eq 403) $r.Content
$r = Req 'POST' "/api/tools/$newTid/offline" $null $admin
Check 'admin 下线 Tool 200' ($r.StatusCode -eq 200) $r.Content

# 4. 权限配置与检查
$r = Req 'POST' "/api/tools/$newTid/permissions" @{ permissions = @(@{ role = 'agent_caller'; can_read = $true; can_call = $false }) } $admin
Check '权限矩阵配置 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'GET' "/api/tools/$newTid/permissions" $null $admin
Check '权限列表 200' ($r.StatusCode -eq 200) $r.Content
$perms = ($r.Content | ConvertFrom-Json).data
Check '权限矩阵回显 agent_caller can_read' ($perms.permissions[0].can_read -eq $true) $r.Content

$agentUid = ((Req 'GET' '/api/auth/me' $null $agent).Content | ConvertFrom-Json).data.id
$r = Req 'POST' "/api/tools/$newTid/permissions" @{ user_id = $agentUid; action = 'call' } $admin
Check '用户直授 call 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'GET' "/api/tools/$newTid/permissions/check?user_id=$agentUid&action=call" $null $admin
Check '权限检查 allowed=true' (($r.Content | ConvertFrom-Json).data.allowed -eq $true) $r.Content
$r = Req 'DELETE' "/api/tools/$newTid/permissions" @{ user_id = $agentUid; action = 'call' } $admin
Check '撤销直授 200' ($r.StatusCode -eq 200) $r.Content

# 5. 评分评论
$r = Req 'POST' "/api/tools/$salesId/reviews" @{ rating = 5; comment = 'smoke review' } $agent
Check '提交评分评论 200/201（重复评分允许 409）' ($r.StatusCode -in 200,201,409) $r.Content
$r = Req 'GET' "/api/tools/$salesId/reviews" $null $agent
Check '评论列表 200' ($r.StatusCode -eq 200) $r.Content

# 6. 适配任务
$r = Req 'POST' "/api/tools/$newTid/adaptation-tasks" @{ task_type = 'openapi_import'; source_url = 'http://example.com/api.yaml' } $agent
Check '创建适配任务 201' ($r.StatusCode -eq 201) $r.Content
$taskId = ($r.Content | ConvertFrom-Json).data.task_id
$r = Req 'GET' "/api/tools/$newTid/adaptation-tasks" $null $agent
Check '适配任务列表 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'GET' "/api/tools/adaptation-tasks/$taskId" $null $agent
Check '查询单个适配任务 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'PATCH' "/api/tools/adaptation-tasks/$taskId" @{ status = 'running' } $agent
Check '更新适配任务状态 200' ($r.StatusCode -eq 200) $r.Content

# 7. MCP 网关：发现 + 调用 + 403
$r = Req 'GET' '/mcp/tools' $null $agent
Check 'agent 发现工具 200' ($r.StatusCode -eq 200) $r.Content
$names = (($r.Content | ConvertFrom-Json).data.tools | ForEach-Object { $_.name })
Check '发现列表含 query_sales' ($names -contains 'query_sales') "names=$($names -join ',')"
Check '发现列表不含 delete_customer（无权限）' ($names -notcontains 'delete_customer') "names=$($names -join ',')"

$r = Req 'GET' '/mcp/tools' $null $restricted
$rnames = (($r.Content | ConvertFrom-Json).data.tools | ForEach-Object { $_.name })
Check 'restricted 发现列表为空' ($rnames.Count -eq 0) "names=$($rnames -join ',')"

$r = Req 'POST' '/mcp/tools/query_sales/call' @{ arguments = @{ month = '2026-08' } } $agent
Check 'agent 调用 query_sales 成功' ($r.StatusCode -eq 200 -and ($r.Content -match 'total_sales')) $r.Content
$r = Req 'POST' '/mcp/tools/delete_customer/call' @{ arguments = @{ customer_id = 1 } } $agent
Check 'agent 调用 delete_customer 403' ($r.StatusCode -eq 403) $r.Content
$r = Req 'POST' '/mcp/tools/not_exist_tool/call' @{ arguments = @{} } $agent
Check '调用不存在工具 404' ($r.StatusCode -eq 404) $r.Content
$r = Req 'POST' '/mcp/tools/query_sales/call' @{ arguments = @{ month = '2026-08' } } $restricted
Check 'restricted 调用 query_sales 403' ($r.StatusCode -eq 403) $r.Content
$r = Req 'GET' '/api/mcp-config' $null $agent
Check 'MCP 接入配置 200' ($r.StatusCode -eq 200) $r.Content

# 8. 审计
$r = Req 'GET' '/api/audit/logs?page=1&page_size=5' $null $admin
Check 'admin 查询审计日志 200' ($r.StatusCode -eq 200) $r.Content
$auditTotal = ($r.Content | ConvertFrom-Json).data.total
Check '审计日志含刚产生的调用记录' ($auditTotal -gt 0) "total=$auditTotal"
$r = Req 'GET' '/api/audit/logs' $null $agent
Check 'agent 查询审计日志 403' ($r.StatusCode -eq 403) $r.Content
$r = Req 'GET' '/api/audit-logs?page=1&page_size=5' $null $admin
Check '旧路径 /api/audit-logs 仍兼容 200' ($r.StatusCode -eq 200) $r.Content

# 9. 指标与统计
$r = Req 'GET' '/api/metrics' $null $admin
Check 'admin 查询 /api/metrics 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'GET' '/api/metrics' $null $agent
Check 'agent 查询 /api/metrics 403' ($r.StatusCode -eq 403) $r.Content
$r = Req 'GET' '/api/analytics/overview?days=7' $null $admin
Check '统计概览 200' ($r.StatusCode -eq 200) $r.Content
$r = Req 'GET' '/api/analytics/alerts' $null $admin
Check '告警列表 200' ($r.StatusCode -eq 200) $r.Content

# 10. CORS 预检（前端 5173 端口）
$r = Req 'OPTIONS' '/api/auth/login' $null $null @{ Origin = 'http://localhost:5173'; 'Access-Control-Request-Method' = 'POST' }
Check 'CORS 预检 204 且允许来源' ($r.StatusCode -eq 204 -and $r.Headers['Access-Control-Allow-Origin'] -eq 'http://localhost:5173') "status=$($r.StatusCode)"

Write-Host "`n=== 结果: $script:pass PASS, $script:fail FAIL ===" -ForegroundColor Cyan
if ($script:fail -gt 0) { $script:failures | ForEach-Object { Write-Host "  FAIL: $_" -ForegroundColor Red }; exit 1 }
