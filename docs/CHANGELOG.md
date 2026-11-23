# 变更记录

## Unreleased

### 诚实机制接线
- 歧义暂停接入主循环：悬空指代直接 `halt`，不执行工具。
- 形态观测接入：自我保证/投降表述产生 warn；自我保证仍触发 Gate 加严。
- 测试计划三态接入：解析 `[TEST PLAN]` 并在回合末 `Finalize`。
- 权限分级接入：`AuthorityOf` + `core.Authorize`；不可逆动作永不记忆授权。
- 失败账本落盘 `%LOCALAPPDATA%\HB\ledger.jsonl`，跨回合 Top-3 检索注入（标注为历史）。

### 命令与界面
- 新增 `hb gc`、`hb export --redact`、`hb upgrade [--check]`（SHA256 校验 + 自替换）。
- TUI `/verify` `/stats` `/sessions` `/context` `/compact` 真正可用。
- TUI `?` 快捷键帮助页、首次使用引导。
- 文档一致性测试扩展到命令表与快捷键表。

### 测试与安全
- 回放样本扩充到 12 个，覆盖 done / untested / denied / halt 与证据缺失路径。
- 新增 `FuzzDiff`、`FuzzMarkdown`；SSE 模糊测试保留。
- 新增二进制级 e2e（真实进程 + 真实 HTTP 模型桩）：`scripts/e2e.ps1`。
- `govulncheck` 清零：x/text → v0.39.0、goldmark → v1.7.17。
- 性能基线填入本机实测值。

### 真实模型测试修复（DeepSeek Flash）

- Gate 只判定与本次改动相关的证据：探索过程中失败的 shell 命令、空目标的参数错误不再永久阻断完成。
- 自我保证正则收紧：`未通过` / `无法通过` / `尚未通过` 不再被误判；保留“测试全部通过”等模式。
- 输出预算独立为 `MaxOutputTokens`（默认 65536，且不超过上下文窗口一半）；`finish_reason=length` 且无工具调用时按 `failed` 结束，不再伪装成 `untested`。
- 支持 DeepSeek 思考模式：解析并回传 `reasoning_content`；新增 `reasoning_effort` 配置（low/high/max）。
- 修复 `RequiredFor`：未知文件类型（如 `.html`）与 `command` 同时存在时只要求 command，导致 HTML + shell 任务被 `evidence_kind` 误拒。
- 缺少 `[SELF]` 自评块时不再立即终止回合，改为拒执行当前步并给两次补救；每次有效自评后计数清零。
- 单回合工具调用上限从 40 提到 100，触发时写入 Receipt。
- `hb approve` 与 `hb run` 的配置对齐（自评、歧义、账本、计划、思考强度、审批）；TUI 传入 `PlanFirst` 与上下文窗口。
- 检查点与仓库地图：主目录自动跳过；排除 AppData / OneDrive / 包缓存；检查点上限 4000 文件或 128MB，超出截断并提示。
- 失败重复检测只统计当前回合，历史账本不再阻断新回合。
- 上下文窗口改为可配置：`context.window` 支持 300k / 600k / 960k，默认 960k；CLI、TUI、approve 统一使用该值。
- shell 工具结果附带退出码，避免模型把“空输出”误判为命令失败。
- system prompt 明确写出 diff / command / build / test 证据来源、`request_done` 与 `declare_untested` 参数格式。

### 文档
- README 精简为入口页：保留价值主张、快速开始、特性、机制、命令、安装与论文链接，
  详细内容移入 `docs/`。
- 新增指导论文 [`docs/paper/constructive-closure.pdf`](paper/constructive-closure.pdf) 及说明。
- 吉祥物改为去黑边的透明背景 `assets/mascot.png`。
- 补齐 `docs/troubleshooting.md` 与 `docs/frozen-scope.md`，不再是指向空文档的链接。
- 文档一致性测试覆盖 `docs/commands.md` 的命令表与快捷键表，并校验 README 中的命令真实存在。

## v0.1.0（规划中）
- 阶段 1–13 实现：core 状态机与 Gate、三套模型协议（Chat Completions / Anthropic Messages / Responses）、
  沙箱工具集、非交互 CLI、会话落盘与哈希链、三态探针、Bubble Tea TUI 与富渲染、
  审批往返与会话恢复、结构化日志与脱敏、错误码、goreleaser / winget / scoop 分发清单。
- 接地谓词、验证器分级、运行/通过分离、原文哈希、形态观测量等机制依据项目指导论文落地。

## Unreleased
- 全量骨架与逐阶段实现推进中；详细范围见 README 与 `docs/architecture.md`。
