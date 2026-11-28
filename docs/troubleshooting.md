# 故障排查

按现象查。错误码全表见 [`errors.md`](errors.md)，退出码见 [`exit-codes.md`](exit-codes.md)。

## 第一次运行

推荐顺序：

```powershell
hb doctor
hb init
hb probe
hb run "读一下 README.md 并总结"
```

- `hb doctor` 检查配置、密钥变量、端点可达性、工作区写权限、会话目录与终端，最后给出“建议下一步”。
- `hb init` 结束时会问“测试连接? [Y/n]”，发一个极小的真实请求验证端点/密钥/模型；失败不会丢配置。
- 无 TTY（管道、CI、winget 安装后的探测）时，`hb` 无参数只打印帮助，不会进入设置向导。
- 上次任务异常中断时，运行 `hb` 会提示“回车继续 / n 新建”；非交互场景用 `hb resume --last`。
- 如果端点不支持 `HEAD`，`hb doctor` 会自动回退到 `GET`；收到任何 HTTP 响应都算“可达”。

## 无密钥 / 认证失败（退出码 6）

```powershell
hb doctor
```

1. `api_key_env` 必须是变量名（如 `HB_API_KEY`），不是 `sk-` 开头的密钥值。
2. Windows 推荐 `. .\set-key.ps1` 写入当前用户；临时使用 `$env:HB_API_KEY="..."`。
3. 新开终端后再试；`hb config list` 只显示变量名与 `***`。
4. 模型 API 返回 Authentication Fails 时，`hb` 会给出三步检查清单。

## 卡在“思考中”

- `Esc` 中断当前回合；连续 `Ctrl+C` 退出。
- 端点不可达会以退出码 6 结束；检查 `base_url`、代理与防火墙。
- 调大超时：`--timeout 20m`，或直接编辑配置文件的 `timeout` 字段。
- 长任务可用 `/context` 查看上下文占用。

## 启动 TUI 后没有任何反应 / 内存暴涨

通常是从用户主目录（如 `C:\Users\<name>`）启动，旧版本会尝试把整个主目录做检查点。
当前版本会自动跳过主目录检查点与仓库地图；仍建议在具体项目目录运行：

```powershell
cd D:\code\your-project
hb
```

如果旧进程仍占内存，先关闭对应窗口再重启。主目录场景可看到提示：
`当前是用户主目录，已跳过检查点；建议在具体项目目录运行 hb`。

## 终端乱码 / 颜色异常

- Windows 先执行 `chcp 65001`，优先使用 Windows Terminal。
- 关闭颜色：`--no-color`，或设置环境变量 `NO_COLOR=1`。
- TUI 需要 TTY；管道或 CI 中请用 `hb run --json`，不要用 `hb chat`。

## 审批弹不出来

- `hb run` 是非交互模式，无法弹审批框；默认 `approval.exec=ask`，需要批准的操作会被拒绝。
- 受信任环境可配置 `hb config set approval.exec auto`；写操作对应 `approval.write`。
- 高危命令始终需要单次授权：在 TUI 中确认，或改写命令避免高危操作。

## 一直是 `untested` / 退出码 3

- 模型只回答了文本，没有调用 `request_done`，也没有调用 `declare_untested`。
- 让模型先产生可验证动作（构建、测试），再申请完成。
- `hb sessions show <id>` 默认显示最近 50 条事件（`--tail N` 调整）；“未验证”不等于“失败”。

## 申请完成被拒（退出码 4）

按 `gate_deny` 的规则处理：

| 规则 | 处理 |
|---|---|
| `evidence_kind` | 缺少要求的验证类型：跑 `verify_build` / `verify_test` / `verify_vet` |
| `coverage` | 有改动没被证据覆盖：验证全部被改文件或整个项目 |
| `evidence_failed` | 验证器退出码非 0：先修失败再申请 |
| `provenance` | 证据来源不可接地：必须是本次真实运行的验证器输出 |

```bash
hb sessions diag <id>
```

## `hb verify` 报证据链断裂（退出码 5）

- 输出会给出会话 ID 与第几条记录断裂。
- 常见原因：手工编辑了会话 jsonl、磁盘写入不完整。
- 该会话不应再作为完成依据；用 `hb export` 保留现场后删除或隔离。

## 编辑失败：锚点不唯一 / 需要先读取

- `edit_file` 要求 `old_string` 唯一；补上更多上下文，或改用 `apply_patch`。
- 写文件前必须先 `read_file`；外部并发改动会报 `ErrChangedSinceRead`，重新读取后再改。
- 这两个限制是故意的：不做模糊匹配，避免改错位置。

## 升级提示文件被占用

- Windows 上运行中的旧进程会锁住 `hb.exe`。
- `hb upgrade` 会先把旧文件改名为 `hb.exe.old`，新文件立即就位；关闭旧窗口后，
  下次启动会自动清理 `.old`。
- 若手动覆盖失败，关闭所有 `hb` 窗口后重试。
- 需要回滚时按 README「校验与回滚」操作；不要先删 `hb.exe.old`，它是升级失败时的恢复副本。

## `hb probe` 不通过

`hb probe` 使用脚本化模型离线运行，不依赖网络或密钥。任一探针不符合预期，说明 Gate 行为出现了回归：

```bash
hb probe
go test ./internal/engine -run TestReplayCases -v
```

把失败样本与 `gate_rule` 一起记录到 issue。
