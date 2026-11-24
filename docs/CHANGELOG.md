# 变更记录

## Unreleased

### TUI 日用小交互

- 大段粘贴折叠为 `[粘贴 N KB]`，提交时还原。
- `Ctrl+Home` / `Ctrl+End` 到顶/到底；`Ctrl+R` 只读重放当前会话。
- `Shift+Enter` 换行，保留 `Alt+Enter`。
- 普通审批禁用输入框；高危审批保留 `yes` 输入并显示红色边框/等待计时。
- 状态栏与本轮面板显示证据数；状态栏显示会话 ID 前 10 位。
- `Ctrl+C` 空闲时连按两次退出；`Ctrl+O` 优先展开审批长命令，否则切换最后一个工具块。
- info 通知 3 秒淡出；`/theme`、`/reload`、真正的 `/compact` 与 `/reset` 可用。
- `/context` 显示 system / HB.md / 历史 / 工具结果的近似拆分；上下文低于 15% 时状态栏转警告色。

### 富渲染

- 助手 Markdown 代码块默认加行号；超过 200 行折叠，Ctrl+O 展开。
- 工具 diff 显示旧/新行号，保留 3 行上下文与 `+`/`-` 前缀。
- 图片降级为 `[图片: alt (URL)]`，长 base64 折叠为 `[base64 NKB]`。
- 支持 OSC 8 超链接的终端启用链接；其余保留可复制 URL。
- 40 个 Markdown golden 快照（10 内容 × 4 宽度），支持 `UPDATE_SNAPSHOTS=1`。
- 200 块 LRU 渲染缓存；200 行代码实测 35.5ms，暂未达到 30ms 目标。

### 引擎与工具

- `hb run --diff` 把工作区 git diff 注入任务上下文。
- `tools.d/*.json` 自定义工具：直接 exec、参数模板、Risk、schema 校验、超时/输出上限。
- `web_fetch`：默认关闭，域名白名单、私网/回环拒绝、2MB 截断、联网审批。
- `session_end` 事件与 `hb sessions` 异常退出标记。
- 单会话超过 50MB 自动分片到 `id-2.jsonl`，Load/List/Delete/AllReceipts 合并分片。
- 同一模型步的只读工具并行执行；写/exec/net 仍串行，事件与证据按原顺序产出。
- Registry 输出真实声明，注册时校验 schema 与重名。
- 删除 `RunChat`/`RunResume` 死代码。

### 可观测与运维

- `hb run --profile` 输出总耗时、上下文装配、流式、首 token、工具耗时。
- `hb doctor --fix` 重建损坏配置；`hb help <topic>` 提供 run/config/evidence/tools/exit-codes。
- `hb upgrade --check` 24 小时缓存；`HB_NO_UPDATE_CHECK=1` 可关闭检查。
- 崩溃日志统一写入 `HB/logs/crash`，不再污染工作区。
- 二进制 e2e 增至 5 个（含无密钥退出 6、无证据 denied 退出 4）；CI 增加 `-count=10` 回归。
- core 覆盖率 82.9% → 95.3%；补 labels/ledger/evidence/session 测试；`apply_patch` 工具动作显示“应用补丁”。
- 沙箱逃逸用例补到 12 个；SSE/Markdown/diff 三个 fuzz 各 60s 本地通过。

## v0.1.1 - 2026-11-24

### 自更新

- `hb upgrade` 默认超时从 2 分钟提高到 15 分钟，可用 `--timeout <dur>` 覆盖。
- 下载失败（网络错误/429/5xx）自动重试 3 次，并在 TTY 下显示下载进度。

### TUI 视觉简化

- 移除 TUI 中的吉祥物、像素球场、颠球/射门动画与 `GOAL!` 状态。
- 顶部改为固定 3 行简洁标题栏（品牌 / 模型 / 工作区），释放的行数全部给对话区。
- 删除 `pitch.go`、`pixel.go`、`pitch_tick.go` 及其测试，移除 `kickFrame` / `pitchTick` 状态。

## v0.1.0 - 2026-11-24

### TUI 交互重构（A+B+C）

- 修复高危审批按键返回 nil model 导致 TUI 崩溃；高危卡与按键统一使用请求里的 `HighRisk`，
  `outside_read` 不再被画成普通审批。
- 布局改为实测高度：审批卡、命令面板、多行输入、状态栏不再互相挤掉；80×24 下审批选项与输入框可见；
  终端高度 < 30 行时横幅收缩为 3 行。
- 回合生命周期引入 `runID` 与 `draining`：中断后旧事件被丢弃、旧引擎释放会话锁前新任务排队；
  回合进行中提交自动排队并在结束后发送，`Esc` 清空队列。
- 对话滚动：PgUp/PgDn、Ctrl+U/D、鼠标滚轮；离开底部时不强制跟随。
- 工具输出 `Ctrl+O` 展开/折叠；`edit_file` / `write_file` / `apply_patch` 产出统一 diff，展开后按 +/− 着色。
- 命令面板：`Enter` 执行高亮项（之前执行的是输入原文），需要参数的命令先补全；选中项不会滚出窗口。
- 多行输入（`Alt+Enter`），输入历史以 JSON 落盘并跨启动恢复。
- 审批与帮助页里的 `Ctrl+C` 生效；新回合重置上一回合的 `done/GOAL` 状态；射门动画结束回到颠球。
- 主题贯通：横幅/面板/审批/Markdown 全部走 `theme`，`NO_COLOR` 下 Markdown 使用 `notty`；
  流式 Markdown 增加已完成段落缓存。
- 会话：TUI 每回合续接同一 `session_id`，引擎注入上一轮历史；`/sessions` 选择恢复、
  `/new` 开新会话、`/approve <id>` 在计划所属会话执行。
- 新增 TUI `/diff` `/undo` `/rewind` `/verbose`；`/context` 改用最近一步提示词估算。
- 清理骨架占位：`component` 只保留审批卡，删除空 `render.Cache`、未使用的吉祥物字符画与 4 个未调用函数。

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
- 新增持久化默认工作区 `workdir`：`--cwd` > 配置 > 当前目录；`hb init` 会保存工作区选择；`hb doctor` 按该目录检查可写性。
- 新增严格质量契约 `rigor=strict`：吸收 Prompt-Engineering-for-Developers 的完整实现、边界/安全、最小 diff、来源说明要求；写入后扫描占位符并回传警告。不改变 Gate、证据与审批架构。
- 输出预算支持 `max_output_tokens`：默认 auto，`reasoning_effort=max` 时 128k、其余 64k；修复 max 思考耗尽 64k 后空响应的问题。
- shell 工具结果附带退出码，避免模型把“空输出”误判为命令失败。
- system prompt 明确写出 diff / command / build / test 证据来源、`request_done` 与 `declare_untested` 参数格式。

### 文档
- README 精简为入口页：保留价值主张、快速开始、特性、机制、命令、安装与论文链接，
  详细内容移入 `docs/`。
- 新增设计论文 [`docs/paper/constructive-closure.pdf`](paper/constructive-closure.pdf) 及说明。
- 吉祥物改为去黑边的透明背景 `assets/mascot.png`。
- 补齐 `docs/troubleshooting.md` 与 `docs/frozen-scope.md`，不再是指向空文档的链接。
- 文档一致性测试覆盖 `docs/commands.md` 的命令表与快捷键表，并校验 README 中的命令真实存在。

### v0.1.0 覆盖范围
- 阶段 1–13 实现：core 状态机与 Gate、三套模型协议（Chat Completions / Anthropic Messages / Responses）、
  沙箱工具集、非交互 CLI、会话落盘与哈希链、三态探针、Bubble Tea TUI 与富渲染、
  审批往返与会话恢复、结构化日志与脱敏、错误码、goreleaser / winget / scoop 分发清单。
- 接地谓词、验证器分级、运行/通过分离、原文哈希、形态观测量等机制依据项目设计论文落地。
