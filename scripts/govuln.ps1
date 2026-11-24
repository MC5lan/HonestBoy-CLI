# 漏洞扫描（发布检查项）。
# 用法：powershell -ExecutionPolicy Bypass -File scripts/govuln.ps1
$ErrorActionPreference = 'Stop'
$goBin = Join-Path $env:USERPROFILE 'go\bin'
if (Test-Path $goBin) { $env:PATH = "$goBin;$env:PATH" }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw '需要在 PATH 中找到 Go 1.27+'
}
Set-Location (Join-Path $PSScriptRoot '..')
if (-not (Get-Command govulncheck -ErrorAction SilentlyContinue)) {
    go install golang.org/x/vuln/cmd/govulncheck@latest
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
govulncheck ./...
exit $LASTEXITCODE
