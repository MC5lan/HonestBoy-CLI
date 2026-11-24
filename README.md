# Honest Boy CLI (`hb`)

<p align="center">
  <img src="assets/mascot.png" width="300" alt="Honest Boy 吉祥物">
</p>

<p align="center">
  <a href="https://github.com/MC5lan/HonestBoy-CLI/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/MC5lan/HonestBoy-CLI/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white">
  <img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue">
  <img alt="Status" src="https://img.shields.io/badge/status-v0.1.0-blue">
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux-lightgrey">
</p>

<p align="center"><strong>完成与否，不听模型说了什么，只看验证器跑出了什么。</strong></p>

`hb` 是一个本地 AI 编程智能体 CLI。它在工作区里读写代码、执行命令、跑构建与测试，
但只有 Gate 认可的验证器证据才能把回合标记为 `done`。没有证据的“我修好了”会被拒绝。

> 模型：已修复。
>
> `hb`：`✗ 还不能算完成: 还有改动没验证：README.md` → 退出码 `4`
>
> 模型可以继续跑构建/测试补齐证据，也可以明确弃权。两条路都比假完成诚实。

## 亮点

| 能力 | 说明 |
|---|---|
| **证据门禁** | `done` 必须由本次真实运行、通过、可接地的 `build` / `test` / `vet` 证据支撑 |
| **可审计** | 证据带原文哈希，Receipt 串成哈希链；`hb verify` 定位篡改 |
| **真工具** | 读/写/精确编辑、`apply_patch`、搜索、shell、构建/测试/静态检查 |
| **长任务** | 上下文压缩、仓库地图、每轮检查点、`hb rewind` 代码回退 |
| **工作流** | 计划模式 + `hb approve`、`--auto-commit`、`hb undo` / `hb diff` |
| **质量契约** | `rigor=strict` 要求完整实现、边界/安全、最小 diff，并检查占位符 |
| **TUI** | 简洁标题栏、可滚动对话、右侧可观测面板、`/` 命令面板、`?` 帮助页 |
| **模型** | OpenAI Chat、Anthropic Messages、OpenAI Responses 三套协议 |
| **本地** | 会话只落本机，无遥测；密钥只从环境变量读取 |

## 快速开始

先按下方[安装](#安装)把 `hb` 放进 PATH，然后：

```bash
hb init                            # 首次配置：端点、模型、密钥环境变量名
hb probe                           # 无需密钥：先验证 Gate 拦不拦得住虚报
hb run "修复 go test ./... 的失败"   # 单次任务
hb                                 # TUI（需要 TTY）
```

## 工作方式

```text
模型 ──▶ 工具 ──▶ 沙箱 ──▶ 验证器证据 ──▶ Gate ──▶ done
                                        │
                     缺证据 / 失败 / 覆盖不到 └──▶ 拒绝（退出码 4）
```

- 只有 `Provenance=observed`、`Ran=true`、一级验证器、`ExitCode>=0`、`OK=true`、原文哈希匹配，证据才能接地。
- 验证器没运行属于“证据缺失”，不冒充成功，也不冒充失败。
- Gate 是唯一能写 `done` 的组件；状态机里没有“动作 → done”的边。

机制细节见 [`docs/architecture.md`](docs/architecture.md) 与 [`docs/evidence-format.md`](docs/evidence-format.md)。

## 常用命令

| 命令 | 说明 |
|---|---|
| `hb` / `hb chat` | 启动 TUI（需要 TTY） |
| `hb run <prompt>` | 单次执行；`--plan` 先出计划，`--auto-commit` 自动提交 |
| `hb probe` | 三态探针，离线验证 Gate |
| `hb verify` / `hb stats` | 校验证据链 / 诚实度统计 |
| `hb sessions` / `hb resume <id>` | 会话列表 / 恢复 |
| `hb approve <id>` | 批准计划并执行 |
| `hb init` / `hb doctor` / `hb config` | 初始化 / 环境自检 / 配置 |
| `hb upgrade` / `hb gc` / `hb export` | 自更新 / 清理 / 脱敏导出 |

完整参数与 TUI 快捷键见 [`docs/commands.md`](docs/commands.md)。

## TUI

- **顶部**：固定 3 行简洁标题栏（品牌 / 模型 / 工作区），不显示吉祥物或球场动画，空间全部留给对话区。
- **中部**：流式 Markdown、代码高亮、工具卡片与可展开的 diff；支持 PgUp/PgDn、Ctrl+U/D 和滚轮回看历史。
- **右侧**：可观测面板（本轮/会话的工具成败、Gate 规则、上下文估算）。
- **底部**：审批卡片、忙碌行、命令面板、多行输入框与状态栏。
- **多轮**：同一会话自动续接上下文；`/sessions` 可选择恢复，`/new` 开始新会话。
- **队列**：回合进行中提交的任务排队，结束后自动发送；`Esc` 中断并清空队列。

| 快捷键 | 行为 |
|---|---|
| `Enter` | 提交任务；命令面板中执行高亮项 |
| `Alt+Enter` | 输入换行 |
| `Ctrl+C` | 中断当前回合；空闲时退出 |
| `Ctrl+L` | 清屏 |
| `Ctrl+O` | 展开/折叠工具输出 |
| `Ctrl+U / Ctrl+D` | 对话区上/下半页 |
| `PgUp / PgDn` | 对话区翻页（滚轮同效） |
| `Esc` | 中断并清空队列 / 关闭提示 |
| `↑ / ↓` | 历史输入 / 命令面板选择 |
| `Tab` | 补全命令 |
| `?` | 快捷键帮助 |
| `/` | 命令面板 |

## 配置速查

配置文件：Windows `%LOCALAPPDATA%\HB\config.json`，Linux `~/.config/HB/config.json`。

| 键 | 默认 | 说明 |
|---|---|---|
| `workdir` | 当前目录 | 默认工作区（沙箱边界）；`--cwd` 可临时覆盖 |
| `context.window` | `960k` | 上下文窗口：`300k` / `600k` / `960k` |
| `reasoning_effort` | 提供方默认 | 思考强度：`low` / `high` / `max` |
| `max_output_tokens` | `auto` | 单次输出预算；`auto` 时 max 思考为 128k、其余 64k |
| `rigor` | `normal` | `strict` 启用严格质量契约与占位符检查 |
| `approval.write` / `.exec` | `auto` / `ask` | 写文件 / 执行命令的审批策略 |

```bash
hb config set workdir D:\code\your-project
hb config set context.window 600k
hb config set reasoning_effort low
hb config set rigor strict
```

## 安装

**Windows（构建 + 安装到 PATH + 启动向导）**

```powershell
git clone https://github.com/MC5lan/HonestBoy-CLI.git
cd HonestBoy-CLI
powershell -ExecutionPolicy Bypass -File .\setup.ps1
```

**Linux（构建 + 安装）**

```bash
git clone https://github.com/MC5lan/HonestBoy-CLI.git
cd HonestBoy-CLI
go build -o bin/hb ./cmd/hb
install -m 755 bin/hb ~/.local/bin/hb   # 确保 ~/.local/bin 在 PATH
```

需要 Go 1.27+。`winget` / `scoop` 清单已就绪，待首个 release 回填 SHA256 并收录。

## 文档

- [架构与机制](docs/architecture.md)
- [命令与快捷键](docs/commands.md)
- [证据格式](docs/evidence-format.md)
- [严格质量契约](docs/rigor.md)
- [故障排查](docs/troubleshooting.md)
- [设计论文（PDF）](docs/paper/constructive-closure.pdf)
- [发布清单](docs/release-checklist.md)

## 开发

```bash
go test ./...        # 单测 + 回放 + 模糊种子
make e2e             # 真实二进制 + HTTP 模型桩
make release-check   # build + vet + test + checkdeps + e2e
```

判定逻辑只放在 `internal/core`（纯函数、仅标准库），UI 不写判定；
依赖方向由 `scripts/checkdeps.*` 机器化检查。贡献约定见 [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md)。

## 状态

核心机制、三套协议、TUI、证据链、检查点、计划模式、严格质量契约与发布脚手架均已实现。
首个正式 release 尚未发布；已知技术债见 [`docs/tech-debt.md`](docs/tech-debt.md)。

Apache-2.0，见 [`LICENSE`](LICENSE)。
