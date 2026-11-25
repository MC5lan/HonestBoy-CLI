# 发布清单（v0.1.x）

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

## 2026-11-24 v0.1.1 发布记录

- 修复 D13：默认 15 分钟、`--timeout`、网络/429/5xx 重试 3 次、TTY 进度。
- `v0.1.1` 已发布：https://github.com/MC5lan/HonestBoy-CLI/releases/tag/v0.1.1
- 线上三个 zip + `checksums.txt` 已核对；winget/scoop 已更新到 0.1.1 并回填真实哈希。
- 用修复后的二进制对真实 GitHub `v0.1.1` 完成下载 → SHA256 → 替换 → 版本确认。
- winget `0.1.0` 与 `0.1.1` 清单已通过 `winget validate`（ManifestVersion 1.6.0，Homepage 改为 PackageUrl）。
- 仍未完成：winget 本地清单安装被本机管理员策略 `LocalManifestFiles` 拦截；scoop 未安装；Linux/arm64 未真机运行。

## 2026-11-25 v0.2.0 发布记录

- 发布提交 `71bedc9`；CI run [45448821770](https://github.com/MC5lan/HonestBoy-CLI/actions/runs/45448821770) 三个 job 全绿。
- tag 与 GitHub Release：https://github.com/MC5lan/HonestBoy-CLI/releases/tag/v0.2.0
- 三个 zip + `checksums.txt` 已上传（state=uploaded）：
  - windows/amd64 `2bb410318249477ac899a27c546900d2805a6e16c3dee27f9e1e7806ac7bdf46`
  - windows/arm64 `7275bca5bf80fe779dc76e472c7e847c693901c56117def531266199aef86f19`
  - linux/amd64 `c8bccc5418175c95aaf5dddac4ffd7238331be46e991225dbf70fd14a17fd5a3`
- winget `0.2.0` 清单与 scoop `hb.json` 已回填真实 SHA256。
- 本机安装：覆盖 `%LOCALAPPDATA%\HB\bin\hb.exe`，旧二进制备份为 `hb.exe.0.1.0-dev.bak`；
  `hb --version` = 0.2.0，`hb probe` 三态通过，`hb upgrade --check` 确认 0.2.0 为最新。
- 说明：goreleaser 在本机下载安装包超时，产物用相同 ldflags 交叉编译 + zip + SHA256 手工构建；
  winget PR 提交与干净 VM 安装/卸载仍未执行。

## 2026-11-25 v0.3.0 发布记录

- 发布提交 `bdf573b`；tag 与 GitHub Release：https://github.com/MC5lan/HonestBoy-CLI/releases/tag/v0.3.0
- 三个 zip + `checksums.txt` 已上传（state=uploaded）：
  - windows/amd64 `7c2c2ae1116aade240881f4c8db8cbe46e9bf6681b5d4cc75f812cd5e6e59d30`
  - windows/arm64 `e0b9d797889040648d62087250efb090cd5b54cd8748776df291786d49bd9126`
  - linux/amd64 `c2e1ff604ec81414a6ca3305aae5e594f080a8e8c0adfb188b7a02ebd4026b50`
- winget `0.3.0` 清单与 scoop `hb.json` 已回填真实 SHA256。
- 本机安装：覆盖 `%LOCALAPPDATA%\HB\bin\hb.exe`，旧 0.2.0 备份为 `hb.exe.0.2.0.bak`。
- 说明：goreleaser 仍未在本机运行；产物用相同 ldflags 手工交叉编译。
- winget 新包 PR 已提交：https://github.com/microsoft/winget-pkgs/pull/488872；
  CLA 已按个人项目签署，等待 wingetbot 校验与维护者合并。
