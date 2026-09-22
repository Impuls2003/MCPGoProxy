$ErrorActionPreference = "Stop"

$DebugDir = $PSScriptRoot

Write-Host "Starting MCP Inspector..." -ForegroundColor Cyan
Write-Host "MCP endpoint: http://127.0.0.1:40242/mcp"
Write-Host ""

Set-Location $DebugDir

npx @modelcontextprotocol/inspector@latest