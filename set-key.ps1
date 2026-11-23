# 设置模型密钥到用户环境变量（持久化，新终端自动生效）
#
# 推荐用法（在当前窗口执行，确保写入当前用户）：
#   . .\set-key.ps1 "你的密钥"
# 或者：
#   powershell -ExecutionPolicy Bypass -File .\set-key.ps1 "你的密钥"
#
# 不带参数则交互式密文输入。

param(
    [string]$KeyEnv = "HB_API_KEY",
    [string]$Key
)

$ErrorActionPreference = 'Stop'
try { chcp 65001 > $null } catch {}
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}

if ([string]::IsNullOrWhiteSpace($Key)) {
    $secure = Read-Host "请输入 $KeyEnv 的密钥（输入时不显示）" -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { $Key = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
}
if ([string]::IsNullOrWhiteSpace($Key)) { throw '密钥不能为空' }

# 1) 注册表直写（用户级持久化）
New-ItemProperty -Path 'HKCU:\Environment' -Name $KeyEnv -Value $Key -PropertyType String -Force | Out-Null
# 2) .NET API 再确认
[Environment]::SetEnvironmentVariable($KeyEnv, $Key, 'User')
# 3) 当前进程立即生效
Set-Item -Path "Env:$KeyEnv" -Value $Key

# 严格校验：三处都必须读到同一个值
$regVal  = (Get-ItemProperty -Path 'HKCU:\Environment' -Name $KeyEnv -ErrorAction Stop).$KeyEnv
$userVal = [Environment]::GetEnvironmentVariable($KeyEnv, 'User')
$procVal = Get-Item -Path "Env:$KeyEnv" -ErrorAction SilentlyContinue
if ($regVal -ne $Key -or $userVal -ne $Key) {
    throw "写入校验失败：注册表=[$regVal] 用户作用域=[$userVal]。请在当前 PowerShell 窗口用 '. .\set-key.ps1' 方式执行。"
}

Write-Host "OK 密钥已写入 $KeyEnv（注册表 + 用户作用域 + 当前进程）"
Write-Host "OK 新开的 PowerShell / CMD 会自动读取"
Write-Host "提示：配置文件只保存变量名 $KeyEnv，不存密钥值"
