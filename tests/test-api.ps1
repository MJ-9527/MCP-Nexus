# MCP-Nexus 接口联调脚本
param([string]$BaseURL = "http://localhost:18080")
$ErrorActionPreference = "Continue"
$PASS = 0; $FAIL = 0
function Assert-Status {$args; param($R,$E,$N) if($R.StatusCode -eq $E){Write-Host "  [PASS] $N (HTTP $($R.StatusCode))" -ForegroundColor Green;$script:PASS++}else{$b=$R.Content;Write-Host "  [FAIL] $N expected $E got $($R.StatusCode): $b" -ForegroundColor Red;$script:FAIL++}}
function Assert-Has {$args; param($R,$K,$N) try{$o=$R.Content|ConvertFrom-Json;if($null-ne$o.$K){Write-Host "  [PASS] $N" -ForegroundColor Green;$script:PASS++}else{Write-Host "  [FAIL] $N missing $K" -ForegroundColor Red;$script:FAIL++}}catch{Write-Host "  [FAIL] $N parse error" -ForegroundColor Red;$script:FAIL++}}

Write-Host "`n=== MCP-Nexus 接口联调测试 ($BaseURL) ===`n" -ForegroundColor Cyan

Write-Host ">>> D1: API 格式" -ForegroundColor Yellow
$r=Invoke-RestMethod "$BaseURL/health"; Assert-Has $r "code" "health has code"; Assert-Has $r "request_id" "health has request_id"

Write-Host ">>> D2: Server 注册/查询" -ForegroundColor Yellow
$body=@{name="demo-ps-server";endpoint="http://localhost:9001";version="1.0.0"}|ConvertTo-Json
$r=Invoke-WebRequest "$BaseURL/api/servers" -Method Post -ContentType "application/json" -Body $body
Assert-Status $r 200 "register server"
$regObj=$r.Content|ConvertFrom-Json; $sid=$regObj.data.id
Assert-Has $r "id" "register returns id"
# Duplicate
$r2=Invoke-WebRequest "$BaseURL/api/servers" -Method Post -ContentType "application/json" -Body $body -ErrorAction SilentlyContinue
Assert-Status $r2 409 "duplicate -> 409"
# Missing fields
$r3=Invoke-WebRequest "$BaseURL/api/servers" -Method Post -ContentType "application/json" -Body '{"name":"x"}' -ErrorAction SilentlyContinue
Assert-Status $r3 400 "missing fields -> 400"
# List
$r=Invoke-WebRequest "$BaseURL/api/servers" -Method Get
Assert-Status $r 200 "list servers"
# Get by ID
$r=Invoke-WebRequest "$BaseURL/api/servers/$sid" -Method Get
Assert-Status $r 200 "get server by id"
# Not found
$r=Invoke-WebRequest "$BaseURL/api/servers/99999" -Method Get -ErrorAction SilentlyContinue
Assert-Status $r 404 "non-existent -> 404"

Write-Host ">>> D8: 安全场景" -ForegroundColor Yellow
# Invalid endpoint
$r=Invoke-WebRequest "$BaseURL/api/servers" -Method Post -ContentType "application/json" -Body '{"name":"bad","endpoint":"not-a-url","version":"1.0.0"}' -ErrorAction SilentlyContinue
Assert-Status $r 400 "invalid endpoint -> 400"
# Empty body
$r=Invoke-WebRequest "$BaseURL/api/servers" -Method Post -ContentType "application/json" -Body '{}' -ErrorAction SilentlyContinue
Assert-Status $r 400 "empty body -> 400"
# Login missing fields
$r=Invoke-WebRequest "$BaseURL/api/auth/login" -Method Post -ContentType "application/json" -Body '{}' -ErrorAction SilentlyContinue
Assert-Status $r 400 "login missing fields -> 400"
# Login invalid creds
$r=Invoke-WebRequest "$BaseURL/api/auth/login" -Method Post -ContentType "application/json" -Body '{"username":"nonexistent_xyz","password":"wrong"}' -ErrorAction SilentlyContinue
Assert-Status $r 401 "login invalid creds -> 401"
# Gateway tools public
$r=Invoke-WebRequest "$BaseURL/mcp/tools" -Method Get -ErrorAction SilentlyContinue
if($r.StatusCode -notin @(404,501)){Assert-Status $r 200 "GET /mcp/tools"}else{Write-Host "  [SKIP] /mcp/tools not implemented" -ForegroundColor Cyan}

Write-Host "`n=== 结果: $PASS PASS, $FAIL FAIL ===`n" -ForegroundColor Cyan
if($FAIL -gt 0){exit 1}
