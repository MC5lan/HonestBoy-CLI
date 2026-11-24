# Honest Boy CLI 一键安装 + 启动
# 用法（在项目目录里）：
#   powershell -ExecutionPolicy Bypass -File .\setup.ps1
#
# 做四件事：构建 -> 安装到用户 PATH -> 修正填错的配置 -> 启动 TUI

$ErrorActionPreference = 'Stop'
chcp 65001 > $null
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$repo = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host '==> 1/4 构建 hb'
$goCmd = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCmd) {
    throw '没找到 Go，请安装 Go 1.27+ 并加入 PATH。'
}
Push-Location $repo
go build -o bin\hb.exe ./cmd/hb
if ($LASTEXITCODE -ne 0) { throw '构建失败' }
Pop-Location

Write-Host '==> 2/4 安装到用户目录并加入 PATH'
$installDir = Join-Path $env:LOCALAPPDATA 'HB\bin'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item (Join-Path $repo 'bin\hb.exe') (Join-Path $installDir 'hb.exe') -Force
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $userPath) { $userPath = '' }
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $installDir), 'User')
}
$env:PATH = "$installDir;$env:PATH"
Write-Host "    已安装到 $installDir"
Write-Host '    新开的终端也能直接用 hb 命令了'

Write-Host '==> 3/4 检查配置与密钥'
$cfgPath = Join-Path $env:LOCALAPPDATA 'HB\config.json'
$keyEnv = 'HB_API_KEY'
if (Test-Path $cfgPath) {
    $cfg = Get-Content $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($cfg.api_key_env) { $keyEnv = $cfg.api_key_env }
    # 如果配置里误填了密钥值，提取出来并改成变量名
    if ($cfg.api_key_env -like 'sk-*') {
        $key = $cfg.api_key_env
        $cfg.api_key_env = 'HB_API_KEY'
        $keyEnv = 'HB_API_KEY'
        $cfg | ConvertTo-Json -Depth 12 | Set-Content $cfgPath -Encoding UTF8
        Write-Host '    发现 api_key_env 填成了密钥值，已自动修正为 HB_API_KEY'
        # 用专门的脚本写入密钥（注册表 + 用户作用域 + 当前进程，带校验）
        & (Join-Path $repo 'set-key.ps1') -KeyEnv $keyEnv -Key $key
        Write-Host '    安全提醒：该密钥曾出现在截图里，建议去服务商处吊销重发'
    } else {
        Write-Host "    api_key_env = $keyEnv，配置正常"
    }
} else {
    Write-Host '    还没有配置文件，先用 hb init 完成设置'
}

# 无论配置如何，都检查密钥是否真的存在；缺失就提示用 set-key.ps1 设置
$keyNow = [Environment]::GetEnvironmentVariable($keyEnv, 'User')
if ([string]::IsNullOrWhiteSpace($keyNow)) {
    Write-Host ''
    Write-Host "    ! 密钥环境变量 $keyEnv 还没设置" -ForegroundColor Yellow
    Write-Host "    请在当前窗口先运行： . .\set-key.ps1 \"你的密钥\"" -ForegroundColor Yellow
    Write-Host '    （不带参数则交互式密文输入；必须带开头的点，确保写进当前用户）'
    Write-Host ''
} else {
    Write-Host "    密钥已设置（环境变量 $keyEnv）"
}

Write-Host '==> 4/4 启动 TUI'
Write-Host ''
& hb chat
