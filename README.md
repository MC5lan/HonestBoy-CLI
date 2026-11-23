# Honest Boy CLI (`hb`)

<p align="center">
  <img src="assets/preview.png" width="360" alt="Honest Boy 像素吉祥物：侧脸小人颠球">
</p>

<p align="center">
  <a href="https://github.com/MC5lan/HonestBoy-CLI/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/MC5lan/HonestBoy-CLI/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white">
  <img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue">
  <img alt="Status" src="https://img.shields.io/badge/status-v0.1.0--dev-orange">
  <img alt="Platform" src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux-lightgrey">
</p>

<p align="center"><strong>同一个模型，换一个 harness，它虚报完成的概率会变。</strong><br>
Honest Boy 让“没验证就说完成”在结构上发不出去。</p>

`hb` 是一个本地 AI 编程智能体 CLI：模型在沙箱里读写文件、执行命令、跑验证，但**完成权不在模型手里**。
模型只能提交“完成申请”，Gate 只认**本次真实运行且通过**的验证器证据。没证据、证据失败、证据覆盖不到改动，
都会被拒绝；模型可以继续补救，也可以明确弃权——两种结果都比假完成诚实。

> **状态**：`v0.1.0-dev`。核心机制、TUI、三套模型协议、测试与发布脚手架均已就绪；
> 预编译包与包管理器清单待首个正式 release。

---

## 目录

- [为什么需要它](#为什么需要它)
- [核心机制](#核心机制)
- [功能总览](#功能总览)
- [安装](#安装)
- [快速开始](#快速开始)
- [TUI](#tui)
- [命令参考](#命令参考)
- [模型协议](#模型协议)
- [配置](#配置)
- [终态与退出码](#终态与退出码)
- [安全边界与不做的事](#安全边界与不做的事)
- [常见问题](#常见问题)
- [开发](#开发)
- [文档索引](#文档索引)
- [许可证](#许可证)

---

## 为什么需要它

普通 agent 里，“我修好了”只是一段文本；在这里，它是一个需要证据的申请。
下面是一次典型的被拒过程（示意，输出格式与真实事件一致）：

```console
$ hb run "把 README 的标题改一下"
▶ 开始任务: 把 README 的标题改一下
  · 使用工具 读取文件
    ✓ README.md  8毫秒
  · 使用工具 编辑文件
    ✓ README.md  5毫秒
  · 使用工具 申请完成
✗ 还不能算完成: 还有改动没验证：README.md

■ 结果: 未完成 · 用时 3.8秒 · 输入 1240 / 输出 96 tokens

$ echo $?
4
```

模型可以继续跑 `verify_build` / `verify_test` 补齐证据，也可以调用 `declare_untested` 明确说明哪些没做。
不能做的只有一件事：没有证据却拿到 `done`。

---

## 核心机制

```text
模型 ──工具调用──▶ 沙箱工具 ──真实输出──▶ 验证器证据（build / test / vet）
  ▲                                              │
  │                                              ▼
  └──────── 拒绝 / 补救 ◀──── Gate（唯一能写 done）◀──── 证据检查
```

- **真值分离**：证据来源只有 `observed` 可接地；`assumed` / `inferred` 只能作参考。
- **完成权外置**：回合状态机没有“动作 → done”的边；`done` 只能由 Gate 写入。
- **接地谓词**：证据必须同时满足 `Provenance=observed`、`Ran=true`、`Level=一级验证器`、
  `ExitCode>=0`、`OK=true`、`RawHash` 与原文匹配。
- **运行/通过分离**：验证器没跑属于“证据缺失”，既不冒充成功，也不冒充失败。
- **原文哈希**：每条证据带 `raw_hash = sha256(raw)`；Receipt 再用 `PrevHash` 串成哈希链，
  `hb verify` 能逐条定位篡改。
- **流程约束**：自评 `conf=unknown` 阻断写操作；悬空指代直接暂停；写操作前要求测试计划；
  高危命令每次单独授权；失败会写入本地账本，下轮按相似度检索注入。
- **可测量**：`hb stats` 统计 done / untested / denied / halt 与虚报完成率（FCR）；
  `hb probe` 用脚本化模型离线验证 Gate 是否真的拦得住。

机制细节与设计依据见 [`docs/architecture.md`](docs/architecture.md) 和 [`docs/evidence-format.md`](docs/evidence-format.md)。

---

## 功能总览

**诚实与审计**

- Gate 完成门禁 + 四类拒绝规则：证据类型、覆盖范围、证据失败、来源不可接地
- 证据原文与哈希、Receipt 哈希链、`hb verify` 篡改定位
- 三态探针 `hb probe`、诚实度报表 `hb stats`、失败账本跨回合检索
- 歧义暂停、自评阻断、测试计划三态、高危动作单次授权

**编程能力**

- 工具集：读/写/列目录、精确替换、`*** Begin Patch` 补丁、搜索、shell、
  `verify_build` / `verify_test` / `verify_vet`
- 上下文自动压缩（保留 system 与最近 2 轮）、仓库符号地图、每轮 git 状态注入
- 每轮文件快照检查点，`hb rewind` 恢复代码而不丢对话
- 计划模式：只读探索 → 计划写入 Receipt → `hb approve` 后才执行
- git 工作流：`--auto-commit` 自动提交并产出 diff 审阅，`hb undo` / `hb diff`

**界面**

- 常驻像素吉祥物与侧视足球场，颠球、射门、进球动画
- 右侧可观测面板：工具成功/失败、证据数、Gate 规则、上下文用量
- 流式 Markdown、代码高亮、diff 卡片、审批卡片
- `/` 命令面板、`?` 快捷键帮助页、首次使用引导、TUI 内切换模型

**数据与维护**

- 会话 jsonl 落盘 + 文件锁、证据链校验、脱敏导出、过期清理、自更新
- 全本地存储，无遥测、无云同步

---

## 安装

### 从源码（当前推荐）

需要 **Go 1.27+** 与 git。

Windows：

```powershell
git clone https://github.com/MC5lan/HonestBoy-CLI.git
cd HonestBoy-CLI
powershell -ExecutionPolicy Bypass -File .\setup.ps1
```

`setup.ps1` 会构建、安装到 `%LOCALAPPDATA%\HB\bin`、加入用户 PATH、检查配置与密钥，然后启动 TUI。

手动构建：

```powershell
go build -o bin\hb.exe .\cmd\hb
```

Linux：

```bash
git clone https://github.com/MC5lan/HonestBoy-CLI.git
cd HonestBoy-CLI
go build -o bin/hb ./cmd/hb
```

### 包管理器（待首个 release）

清单已就绪（`winget/`、`scoop/`）。需先把清单提交到 winget-pkgs 与 scoop bucket，
并用 release 产物回填 SHA256，下面的命令才可用：

```powershell
# v0.1.0 发布并收录后可用
winget install HonestBoy.hb
scoop install hb
```

### 首次配置

```bash
hb init      # 三步向导：工作区 / 协议与端点 / 密钥环境变量名
hb doctor    # 环境自检：配置、密钥、端点连通性
```

非交互配置：

```bash
hb config set protocol openai-chat
hb config set base_url https://api.deepseek.com
hb config set model deepseek-chat
```

密钥只放环境变量：

```powershell
# PowerShell（持久化、密文输入，推荐）
. .\set-key.ps1

# 仅当前窗口
$env:HB_API_KEY = "你的密钥"
```

```bash
# Linux / macOS
export HB_API_KEY="你的密钥"
```

配置文件里**只保存环境变量名**（`api_key_env`），永不保存密钥值。

---

## 快速开始

```bash
cd your-project

hb init                                # 首次使用：配置端点与密钥
hb probe                               # 不需要密钥：先看 Gate 如何拦住虚报
hb run "修复 go test ./... 的失败"     # 单次任务（非交互）
hb run --plan "重构 internal/foo"      # 先只读探索，产出计划后暂停
hb approve <session-id>                # 批准计划并执行
hb                                     # 有 TTY 时启动 TUI
```

实用组合：

```bash
hb run --auto-commit "实现 X 并跑测试"  # 自动提交 + diff 审阅
hb run --json "跑测试" | tee events.jsonl  # 事件流进 CI
hb probe --json                        # 离线验证 Gate 是否拦得住虚报
```

> **非交互模式提示**：`hb run` 无法弹审批框。默认 `approval.exec=ask`，需要批准的命令会被拒绝。
> 可在受信任环境配置 `hb config set approval.exec auto`，或使用 TUI 交互批准。
> 高危命令（`rm -rf /`、`git push --force` 等）**永远不会**被自动批准。

---

## TUI

`hb`（TTY）或 `hb chat` 启动。界面由四部分组成：

- **顶部**：常驻球场框，像素小人颠球；回合通过 Gate 时射门并显示 `GOAL!`
- **中部**：对话与工具卡片，流式渲染 Markdown / 代码高亮 / diff
- **右侧**：可观测面板（工具成功与失败、证据、Gate 规则、上下文剩余）
- **底部**：忙碌行、审批卡片、`/` 命令面板、输入框、状态栏与快捷键提示

常用快捷键：

| 快捷键 | 行为 |
|---|---|
| `Enter` | 提交任务或执行命令 |
| `Ctrl+C` | 中断当前回合（空闲时退出） |
| `Ctrl+L` | 清空对话显示 |
| `Esc` | 忙碌时中断；空闲时关闭提示 |
| `↑ / ↓` | 翻历史输入 / 在命令面板内选择 |
| `Tab` | 补全斜杠命令 |
| `?` | 打开或关闭快捷键帮助 |
| `/` | 打开命令面板 |

TUI 内命令：

| 命令 | 行为 |
|---|---|
| `/model <name>` | 切换模型并写回配置 |
| `/config` `/path` | 配置摘要 / 完整工作区路径 |
| `/verify` `/stats` `/sessions` | 内联校验证据链 / 统计 / 会话列表 |
| `/context` `/compact` | 上下文占用 / 压缩策略说明 |
| `/setup` `/clear` `/help` `/quit` | 设置提示 / 清屏 / 帮助 / 退出 |

---

## 命令参考

**日常**

| 命令 | 说明 |
|---|---|
| `hb` | 有 TTY 启动 TUI；无 TTY 打印帮助；首次运行进入 `hb init` |
| `hb init` | 首次设置向导（也可从 stdin 驱动） |
| `hb run <prompt>` | 单次执行，按终态返回退出码 |
| `hb chat` | 显式启动 TUI（需要 TTY） |
| `hb doctor` | 环境自检，只诊断不修改 |
| `hb config list` / `get` / `set` / `path` | 配置查看与修改 |
| `hb explain <code>` | 查询错误码（如 `HB-NET-001`） |

**会话与证据**

| 命令 | 说明 |
|---|---|
| `hb sessions` | 会话列表 |
| `hb sessions show <id>` | 查看会话事件与终态 |
| `hb sessions diag <id>` | 诊断 Gate 拒绝原因分布 |
| `hb sessions rm <id>` | 删除会话 |
| `hb resume <id>` | 在 TUI 中恢复会话（需要 TTY） |
| `hb verify [--session <id>]` | 重算 Receipt 哈希链，定位断点 |
| `hb stats` | 终态分布与 FCR |
| `hb probe [--json]` | 三态探针：正常 / 诱导 / 施压（离线） |
| `hb export <id> [--redact]` | 导出会话 jsonl 到 stdout |
| `hb gc [--yes]` | 清理 >90 天会话与 >14 天日志（默认预演） |

**代码与撤销**

| 命令 | 说明 |
|---|---|
| `hb diff` | 查看工作区 diff |
| `hb undo` | 撤销最近一次自动提交，保留文件改动 |
| `hb rewind list` | 列出检查点 |
| `hb rewind restore <id>` | 恢复代码到检查点（保留对话） |
| `hb approve <id>` | 批准计划并执行 |

**维护**

| 命令 | 说明 |
|---|---|
| `hb upgrade [--check]` | 下载最新 release、校验 SHA256、替换自身 |
| `hb help` | 打印帮助 |

全局 flag：

```text
--model <name> --protocol <name> --base-url <url> --cwd <dir>
--json --quiet --yes --plan --auto-commit --no-color --debug
--timeout <dur> --max-turns <n> --version --help
```

完整说明见 [`docs/commands.md`](docs/commands.md)。

---

## 模型协议

| 协议 | `--protocol` | 端点 | 备注 |
|---|---|---|---|
| OpenAI Chat Completions | `openai-chat`（默认） | `{base_url}/chat/completions` | 生态最广，DeepSeek / Ollama 等兼容 |
| Anthropic Messages | `anthropic` | `{base_url}/v1/messages` | Claude 及兼容网关 |
| OpenAI Responses | `responses` | `{base_url}/responses` | 语义化 SSE；显式 `store:false` |

自动推断：URL 含 `anthropic` 或 `/v1/messages` → Anthropic；以 `/responses` 结尾 → Responses；
其余 → Chat Completions。三套适配器共用同一个 `llm.Client`；会话状态只落本地，
Responses 适配器显式设置 `store:false`。

---

## 配置

配置文件：

- Windows：`%LOCALAPPDATA%\HB\config.json`
- Linux：`~/.config/HB/config.json`

示例：

```json
{
  "schema_version": 1,
  "protocol": "openai-chat",
  "model": "deepseek-chat",
  "base_url": "https://api.deepseek.com",
  "api_key_env": "HB_API_KEY",
  "shell": "cmd",
  "theme": "auto",
  "max_turns": 20,
  "timeout": "10m",
  "approval": {
    "write": "auto",
    "exec": "ask",
    "net": "ask"
  },
  "context": {
    "budget_ratio": 0.7,
    "keep_recent_turns": 2,
    "hb_md_limit": 16384
  }
}
```

- `api_key` 字段被明确禁止；只允许 `api_key_env`。
- `approval.write|exec|net` 取值 `auto` / `ask`；高危命令不受 `auto` 影响。
- 上下文预算默认窗口的 70%，system 与最近 2 轮永不裁剪。
- `theme` 支持 `auto` / `dark` / `light` / `none`。

---

## 终态与退出码

| 终态 | 含义 | 退出码 |
|---|---|---|
| `done` | 完成，且有可接地证据 | 0 |
| `failed` | 运行失败 | 1 |
| `halt` | 主动暂停（计划模式、自评阻断、歧义等） | 1 |
| `untested` | 明确弃权 / 只回答未申请完成 | 3 |
| `denied` | 申请完成被 Gate 拒绝 | 4 |
| — | 证据链损坏（`hb verify`） | 5 |
| — | 环境问题（无密钥、网络不可用） | 6 |
| — | 超时 | 7 |
| — | 用法错误 | 2 |
| — | 用户 Ctrl-C | 130 |

这是 `hb` 能被 CI 直接使用的关键：**“模型说完成”不是 0，Gate 批准才是 0。**

---

## 安全边界与不做的事

- **沙箱边界**：工作区根目录是硬边界，越界路径直接报 `HB-SBX-003`，不做模糊匹配；
  需要外部文件时，先复制进工作区。沙箱约束的是文件路径，不是操作系统级隔离。
- **审批**：写/执行/联网可配置 `auto` 或 `ask`；高危命令始终单次授权，不提供“永久允许”。
- **密钥**：只从环境变量读取；配置、会话与日志均不含密钥值，导出可加 `--redact`。
- **本地优先**：无遥测、无云同步；只有你配置的模型端点会收到请求。
- **不做的事**：不做 GUI、不做远程执行、不做绝对防作弊、不保证代码正确。
  它保证的是：**没有证据就不能宣称完成**——验证器覆盖不到的错误仍可能漏过。
- 平台：官方产物为 Windows / Linux；Go 1.27 不支持 Windows 7。

详见 [`docs/frozen-scope.md`](docs/frozen-scope.md)。

---

## 常见问题

**为什么结果总是 `untested`？**
模型只回答、没有调用 `request_done`，或主动 `declare_untested`。任务需要可验证的产物时，
让它运行构建/测试后再申请完成。

**为什么申请完成被拒（退出码 4）？**
Gate 拒绝规则有四类：少了验证类型、改动没被覆盖、验证失败、证据来源不可接地。
`hb sessions diag <id>` 可看拒绝分布；按提示补 `verify_build` / `verify_test`。

**`hb` 能离线用吗？**
`hb probe` 完全离线。`hb run` 需要端点，可指向本地 Ollama / vLLM 等兼容网关。

**密钥明明设置了却不生效？**
1) 确认 `api_key_env` 是变量名（如 `HB_API_KEY`），不是 `sk-` 开头的密钥；
2) 用 `. .\set-key.ps1` 写入当前用户；
3) 新开终端，或先执行 `$env:HB_API_KEY="..."`；4) `hb doctor` 复检。

**升级时提示文件被占用？**
Windows 上正在运行的旧进程会锁住二进制。`hb upgrade` 会先把旧文件改名为 `hb.exe.old`；
关闭旧窗口后，下次启动自动清理。

更多排查见 [`docs/troubleshooting.md`](docs/troubleshooting.md)。

---

## 开发

```bash
go build ./...
go test ./...
go vet ./...
```

常用目标：

```bash
make build          # bin/hb.exe
make test           # 全量测试
make bench          # 性能基线
make e2e            # 二进制级端到端（真实进程 + HTTP 模型桩）
make govuln         # govulncheck 漏洞扫描
make release-check  # build + vet + test + checkdeps + e2e
```

目录结构（依赖方向 `cmd → cli/tui/engine → core/llm/tools/store`）：

```text
cmd/hb/           入口与 TUI 装配
internal/core/    纯函数判定：Gate、证据、状态机、歧义、形态、授权、账本
internal/llm/     三套协议适配器与 SSE
internal/tools/   沙箱工具与验证器
internal/engine/  回合主循环、上下文、检查点、自动提交
internal/cli/     命令实现与人类可读输出
internal/tui/     Bubble Tea 界面（不依赖 engine/tools/store）
internal/store/   配置、会话 jsonl、文件锁
e2e/              真实二进制 + 真实 HTTP 的端到端测试
docs/             架构、命令、证据格式、发布清单
```

约束：

- 所有判定逻辑留在 `internal/core`，纯函数、仅标准库；UI 层不写判定。
- `scripts/checkdeps.ps1` / `.sh` 机器化检查反向依赖，违反即失败。
- 命令表与快捷键表有文档一致性测试；改 `--help` 或快捷键必须同步文档。
- 提交信息遵循 [`docs/commit-convention.md`](docs/commit-convention.md)。
- 贡献前请跑 `make release-check`；更多约定见 [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md)。

---

## 文档索引

| 文档 | 内容 |
|---|---|
| [`docs/architecture.md`](docs/architecture.md) | 分层、机制接线、协议层、设计依据 |
| [`docs/commands.md`](docs/commands.md) | 命令、TUI 快捷键、协议与上下文策略 |
| [`docs/evidence-format.md`](docs/evidence-format.md) | 证据六要素、接地谓词、哈希链、jsonl |
| [`docs/exit-codes.md`](docs/exit-codes.md) | 退出码契约 |
| [`docs/errors.md`](docs/errors.md) | `HB-*` 错误码与处置 |
| [`docs/dependencies.md`](docs/dependencies.md) | 依赖清单与漏洞扫描 |
| [`docs/release-checklist.md`](docs/release-checklist.md) | 发布、SHA256 回填、自更新验证 |
| [`docs/tech-debt.md`](docs/tech-debt.md) | 已知技术债与状态 |
| [`docs/CHANGELOG.md`](docs/CHANGELOG.md) | 变更记录 |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | 贡献指南 |
| [`docs/troubleshooting.md`](docs/troubleshooting.md) | 故障排查 |
| [`docs/dogfood-log.md`](docs/dogfood-log.md) | 实际使用中发现的问题 |

## 项目状态

- ✅ 核心诚实机制、三套协议、沙箱工具、CLI、TUI、证据链
- ✅ 会话恢复、检查点/回退、计划模式、自动提交、上下文压缩
- ✅ 12 个回放样本、3 个模糊测试、二进制级 e2e、`govulncheck` 0 可达漏洞
- ⏳ 首个正式 release、winget/scoop SHA256 回填、GitHub 真实升级验证
- 已知技术债：二进制约 18MB（目标 12MB）、token 估算 ±20%，见 `docs/tech-debt.md`

## 许可证

[Apache-2.0](LICENSE)。品牌资源在 `assets/`。
