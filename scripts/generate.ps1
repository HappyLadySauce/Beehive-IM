# Beehive-IM RPC code generation entry (Windows PowerShell)
# Beehive-IM RPC 代码生成入口（Windows PowerShell）
#
# goctl is run inside services/{name}/ with:
#   --proto_path=../..  (protoc include path for repository root)
#   --go_out=. --go-grpc_out=.  (paired with go_package = "./pb" in proto files)
#
# Usage / 用法:
#   .\scripts\generate.ps1
#   .\scripts\generate.ps1 -Service auth
#   .\scripts\generate.ps1 -Service user

param(
    [string]$Service = ''
)

$ErrorActionPreference = 'Stop'

if (-not (Get-Command goctl -ErrorAction SilentlyContinue)) {
    throw 'goctl is not installed or not in PATH. Install: go install github.com/zeromicro/go-zero/tools/goctl@latest'
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$ProtoRoot = Join-Path $RepoRoot 'proto'

if (-not (Test-Path $ProtoRoot)) {
    throw "Proto directory not found: $ProtoRoot"
}

$protoFiles = Get-ChildItem -Path $ProtoRoot -Filter '*.proto' | Sort-Object Name
if ($protoFiles.Count -eq 0) {
    throw "No .proto files found in $ProtoRoot"
}

$matched = @()
foreach ($proto in $protoFiles) {
    $name = $proto.BaseName
    if ($Service -and $name -ne $Service) {
        continue
    }
    $matched += $proto
}

if ($matched.Count -eq 0) {
    throw "No proto file matched service: $Service"
}

foreach ($proto in $matched) {
    $name = $proto.BaseName
    $svcDir = Join-Path $RepoRoot "services\$name"
    $protoRel = (Join-Path (Join-Path '..' '..') (Join-Path 'proto' "$name.proto")) -replace '\\', '/'

    New-Item -ItemType Directory -Force -Path $svcDir | Out-Null

    Write-Host "Generating services/$name from proto/$($proto.Name) ..."

    Push-Location $svcDir
    try {
        & goctl rpc protoc $protoRel `
            --proto_path=../.. `
            --go_out=. `
            --go-grpc_out=. `
            --zrpc_out=. `
            --client=true
        if ($LASTEXITCODE -ne 0) {
            throw "goctl failed for service: $name (exit code $LASTEXITCODE)"
        }
        Write-Host "Done: services/$name"
    } finally {
        Pop-Location
    }
}
