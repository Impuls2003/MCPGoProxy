$ErrorActionPreference = "Stop"

$DebugDir = $PSScriptRoot
$ProjectDir = Split-Path $DebugDir -Parent
$Exe = Join-Path $ProjectDir "mcp_server.exe"

if (-not (Test-Path $Exe)) {
    Write-Host "ERROR: mcp_server.exe not found:" -ForegroundColor Red
    Write-Host $Exe
    Write-Host ""
    Write-Host "Build the project first."
    exit 1
}

Write-Host "Starting MCPGoProxy..." -ForegroundColor Cyan
Write-Host "Project: $ProjectDir"
Write-Host "Executable: $Exe"
Write-Host "HTTP endpoint: http://127.0.0.1:40242/mcp"
Write-Host ""

Set-Location $DebugDir

& $Exe --http

$ExitCode = $LASTEXITCODE

Write-Host ""
Write-Host "MCPGoProxy stopped. Exit code: $ExitCode" -ForegroundColor Yellow