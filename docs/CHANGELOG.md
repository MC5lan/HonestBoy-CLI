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

### 文档
- README 全面重写：30 秒理解、机制图、安装/快速开始、TUI、命令分组、协议、配置、
  终态退出码、安全边界、FAQ、开发与文档索引。
- 补齐 `docs/troubleshooting.md` 与 `docs/frozen-scope.md`，不再是指向空文档的链接。
- 文档一致性测试扩展到 README 的命令表与快捷键表。

## v0.1.0（规划中）
- 阶段 1–13 实现：core 状态机与 Gate、三套模型协议（Chat Completions / Anthropic Messages / Responses）、
  沙箱工具集、非交互 CLI、会话落盘与哈希链、三态探针、Bubble Tea TUI 与富渲染、
  审批往返与会话恢复、结构化日志与脱敏、错误码、goreleaser / winget / scoop 分发清单。
- 接地谓词、验证器分级、运行/通过分离、原文哈希、形态观测量等机制依据项目指导论文落地。

## Unreleased
- 全量骨架与逐阶段实现推进中；详细范围见 README 与 `docs/architecture.md`。
