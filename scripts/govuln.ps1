# 漏洞扫描（发布检查项）。
# 用法：powershell -ExecutionPolicy Bypass -File scripts/govuln.ps1
$ErrorActionPreference = 'Stop'
$env:PATH = (Join-Path $env:USERPROFILE 'go\bin') + ';D:\win7harness\go-toolchain\go\bin;' + $env:PATH
Set-Location (Join-Path $PSScriptRoot '..')
if (-not (Get-Command govulncheck -ErrorAction SilentlyContinue)) {
    $env:GOPROXY = 'https://goproxy.cn,direct'
    go install golang.org/x/vuln/cmd/govulncheck@latest
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
govulncheck ./...
exit $LASTEXITCODE
