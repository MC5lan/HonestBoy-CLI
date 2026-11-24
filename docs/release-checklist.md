# 发布清单（v0.1.0）

发布前必须全绿：

```powershell
make release-check   # build + vet + test + checkdeps + e2e
make govuln          # govulncheck：0 个可达漏洞
make bench           # 与 docs/perf-baseline.md 对比，退化 >20% 需说明
```

## 1. 版本与标签

1. 确认 `docs/CHANGELOG.md` 的 `Unreleased` 已整理到目标版本。
2. 打标签并推送（goreleaser 由标签触发）：

```powershell
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

## 2. 构建发布产物

本地演练（不发布）：

```powershell
goreleaser release --snapshot --clean
```

已用 snapshot 产物验证过：zip 内二进制名、`checksums.txt` 格式与 `hb upgrade` 的资产匹配逻辑一致。

正式发布：

```powershell
$env:GITHUB_TOKEN = "<有 repo 权限的 token>"
goreleaser release --clean
```

产物（见 `.goreleaser.yaml`）：

- `hb_0.1.0_windows_amd64.zip` / `hb_0.1.0_windows_arm64.zip`
- `hb_0.1.0_linux_amd64.zip`
- `checksums.txt`

## 3. 回填包管理器哈希

从 release 下载 `checksums.txt`，把对应 zip 的 SHA256 填到：

- `winget/manifests/h/HonestBoy/hb/0.1.0/HonestBoy.hb.installer.yaml` 的两处 `InstallerSha256`
- `scoop/hb.json` 的两个 `hash`

替换 `REPLACE_WITH_RELEASE_SHA256` 后校验：

```powershell
winget validate --manifest winget/manifests/h/HonestBoy/hb/0.1.0
scoop checkver .\scoop\hb.json
```

## 4. 验证自更新

```powershell
hb upgrade --check
hb upgrade
hb --version
```

`hb upgrade` 会下载 zip、比对 `checksums.txt` 的 SHA256、替换当前二进制；
Windows 下旧文件先改名为 `hb.exe.old`，下次启动自动清理。

## 5. 发布后

- 在 `docs/tech-debt.md` 关闭 D05（清单哈希）与 D06（真实升级验证）。
- 提交 winget/scoop 清单（各自仓库 PR）。
- 记录真实升级结果到 `docs/dogfood-log.md`。

## 2026-11-24 v0.1.0 发布演练记录

已通过：

- CI run `45354988179`（commit `5ff1008`）：`build`（Linux `-race`）、`e2e-windows`、`govuln` 全绿。
- ConPTY 真实 TUI：启动、`?` 帮助、`/` 命令面板、`/config`、Ctrl+C 退出。
- 真实模型闭环：临时 Go 仓库修复 `Add`，读取 → 编辑 → test → build → vet → Gate `done`，
  10.8 秒，输入 12425 / 输出 782 tokens，退出码 0。
- `goreleaser release --skip=publish --clean`（本地 tag `v0.1.0`）：
  三个 zip + `checksums.txt`，SHA256 全部匹配，Windows 解压产物 `--version` / `probe` 正常。
- `hb upgrade --api <local>`：下载 `v0.1.0` 资产、校验 SHA256、替换二进制，
  升级后版本为 `0.1.0`。

尚未完成，不能据此直接宣布正式发布：

- ~~`v0.1.0` tag 仅存在本地，尚未 push。~~ 已 push。
- ~~未创建 GitHub Release。~~ 已发布：https://github.com/MC5lan/HonestBoy-CLI/releases/tag/v0.1.0
- ~~winget/scoop 的 SHA256 仍为占位符。~~ 已按真实 checksums 回填；本机无 winget/scoop 命令，安装未实测。
- `hb upgrade --check` 对真实 release 通过；完整 `upgrade` 因本机到 GitHub 资产链路约 19KB/s、2 分钟超时而失败（D13）。
- 本机未跑 `-race`（无 gcc）；CI run `45355568756` 已覆盖并通过。
