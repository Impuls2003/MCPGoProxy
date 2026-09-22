$ErrorActionPreference = "Stop"

$DebugDir = $PSScriptRoot
$OutputFile = Join-Path $DebugDir "tools-schema.json"

$McpUrl = "http://127.0.0.1:40242/mcp"
$TempRequest = Join-Path $env:TEMP "mcp-request.json"
$TempResponse = Join-Path $env:TEMP "mcp-response.bin"

function Write-Utf8JsonFile {
    param(
        [object]$Object,
        [string]$Path
    )

    $Json = $Object | ConvertTo-Json -Depth 100

    [System.IO.File]::WriteAllText(
        $Path,
        $Json,
        [System.Text.UTF8Encoding]::new($false)
    )
}

function Invoke-McpRequest {
    param(
        [string]$Body,
        [string]$SessionId
    )

    [System.IO.File]::WriteAllText(
        $TempRequest,
        $Body,
        [System.Text.UTF8Encoding]::new($false)
    )

    $CurlArgs = @(
        "-s"
        "-X", "POST"
        $McpUrl
        "-H", "Content-Type: application/json"
    )

    if ($SessionId) {
        $CurlArgs += "-H"
        $CurlArgs += "Mcp-Session-Id: $SessionId"
    }

    $CurlArgs += "--data-binary"
    $CurlArgs += "@$TempRequest"

    $CurlArgs += "--output"
    $CurlArgs += $TempResponse

    & curl.exe @CurlArgs

    if ($LASTEXITCODE -ne 0) {
        throw "curl.exe failed with exit code $LASTEXITCODE"
    }

    if (-not (Test-Path $TempResponse)) {
        throw "MCP server returned no response."
    }

    $Bytes = [System.IO.File]::ReadAllBytes($TempResponse)

    # MCP/HTTP response is UTF-8.
    $ResponseBody = [System.Text.Encoding]::UTF8.GetString($Bytes)

    return $ResponseBody
}

Write-Host "MCP endpoint: $McpUrl" -ForegroundColor Cyan
Write-Host ""

# ------------------------------------------------------------
# 1. initialize
# ------------------------------------------------------------

$InitializeBody = @{
    jsonrpc = "2.0"
    id = 1
    method = "initialize"
    params = @{
        protocolVersion = "2025-03-26"
        capabilities = @{}
        clientInfo = @{
            name = "debuggingTools"
            version = "1.0"
        }
    }
} | ConvertTo-Json -Depth 20

Write-Host "Initializing MCP session..." -ForegroundColor Yellow

[System.IO.File]::WriteAllText(
    $TempRequest,
    $InitializeBody,
    [System.Text.UTF8Encoding]::new($false)
)

$InitHeaders = Join-Path $env:TEMP "mcp-init-headers.txt"
$InitBody = Join-Path $env:TEMP "mcp-init-body.bin"

& curl.exe `
    -s `
    -D $InitHeaders `
    -o $InitBody `
    -X POST `
    $McpUrl `
    -H "Content-Type: application/json" `
    --data-binary "@$TempRequest"

if ($LASTEXITCODE -ne 0) {
    throw "curl.exe failed during initialize."
}

$SessionId = $null

foreach ($Line in Get-Content $InitHeaders) {
    if ($Line -match "^Mcp-Session-Id:\s*(.+)$") {
        $SessionId = $Matches[1].Trim()
        break
    }
}

if (-not $SessionId) {
    throw "Mcp-Session-Id was not returned by MCP server."
}

Write-Host "Session ID: $SessionId" -ForegroundColor DarkGray

# ------------------------------------------------------------
# 2. notifications/initialized
# ------------------------------------------------------------

$InitializedBody = @{
    jsonrpc = "2.0"
    method = "notifications/initialized"
    params = @{}
} | ConvertTo-Json -Depth 10

Invoke-McpRequest `
    -Body $InitializedBody `
    -SessionId $SessionId | Out-Null

# ------------------------------------------------------------
# 3. tools/list
# ------------------------------------------------------------

$ToolsBody = @{
    jsonrpc = "2.0"
    id = 2
    method = "tools/list"
    params = @{}
} | ConvertTo-Json -Depth 10

Write-Host "Requesting tools/list..." -ForegroundColor Yellow

$ToolsResponse = Invoke-McpRequest `
    -Body $ToolsBody `
    -SessionId $SessionId

if (-not $ToolsResponse) {
    throw "tools/list returned an empty response."
}

# ------------------------------------------------------------
# 4. Parse and save
# ------------------------------------------------------------

$Json = $ToolsResponse | ConvertFrom-Json

Write-Utf8JsonFile `
    -Object $Json `
    -Path $OutputFile

Write-Host ""
Write-Host "Tools schema saved:" -ForegroundColor Green
Write-Host $OutputFile
Write-Host ""

if ($Json.result.tools) {
    Write-Host "Tools:" -ForegroundColor Cyan

    foreach ($Tool in $Json.result.tools) {
        Write-Host "  - $($Tool.name)"
    }
}