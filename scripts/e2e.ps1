# 二进制级端到端测试（真实进程 + 真实 HTTP 模型桩）。
# 用法：powershell -ExecutionPolicy Bypass -File scripts/e2e.ps1
$ErrorActionPreference = 'Stop'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw '需要在 PATH 中找到 Go 1.27+'
}
$env:HB_E2E = '1'
Set-Location (Join-Path $PSScriptRoot '..')
go build ./... ; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go test ./e2e -v -count=1
exit $LASTEXITCODE
