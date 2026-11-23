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
