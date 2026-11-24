# 命令参考

| 命令 | 说明 | TTY 依赖 |
|---|---|---|
| `hb` | 无参数打印帮助（有 TTY 时启动 TUI） | 条件 |
| `hb init` | 首次设置向导（工作区 / 协议 / 端点 / 密钥） | 否 |
| `hb run <prompt>` | 单次执行 | 否 |
| `hb chat` | 显式交互（Bubble Tea TUI） | 是 |
| `hb`（无参数） | 有 TTY 时启动 TUI，无 TTY 打印帮助 | 条件 |
| `hb sessions` | 列出/查看/删除会话 | 否 |
| `hb resume <id>` | 恢复会话（TUI 回放历史并可继续） | 是 |
| `hb verify` | 校验证据链 | 否 |
| `hb stats` | 统计 | 否 |
| `hb probe` | 三态探针 | 否 |
| `hb config` | 配置查看/设置 | 否 |
| `hb doctor` | 环境自检 | 否 |
| `hb explain <code>` | 查询错误码 | 否 |
| `hb gc [--yes]` | 清理超过 90 天的会话与超过 14 天的日志；默认预演 | 否 |
| `hb export <id> [--redact]` | 导出会话 jsonl；`--redact` 用统一脱敏规则 | 否 |
| `hb upgrade [--check] [--force] [--timeout <dur>]` | 检查并安装最新 release（SHA256 校验；`--check` 只检查；默认 15 分钟超时，失败自动重试） | 否 |
| `hb undo` | 撤销最近一次自动提交（保留文件改动） | 否 |
| `hb diff` | 查看当前工作区 diff | 否 |
| `hb approve <id>` | 批准计划并执行（先 `hb run --plan`） | 否 |
| `hb rewind list` | 列出检查点（默认最近会话） | 否 |
| `hb rewind restore <id>` | 恢复代码到检查点（对话历史保留） | 否 |
| `hb help` | 打印帮助 | 否 |

全局 flag：`--model` `--protocol` `--base-url` `--cwd` `--json` `--quiet`
`--yes` `--plan` `--auto-commit` `--timeout` `--max-turns` `--no-color` `--debug` `--version` `--help`

`--auto-commit`（Aider 风格）：回合开始前把用户已有脏改动单独提交；回合结束后若有写操作则自动提交，
并产出 `diff_review` 事件（提交号/文件列表/统计）供 TUI 与日志审阅。用 `hb undo` 撤销、`hb diff` 查看。

`--plan` 进入计划模式：只暴露只读工具，模型输出执行计划后回合以 `halt` 暂停并把计划写入 Receipt；
用 `hb approve <session-id>` 批准并按计划执行。

## TUI 内命令

| 命令 | 行为 |
|---|---|
| `/model <name>` | 切换模型并写回配置（失败时保留原模型） |
| `/config` `/path` | 显示配置摘要与完整仓库路径 |
| `/clear` `/new` | 清空显示 / 开始新会话（下一次输入创建新会话） |
| `/verify` | 内联校验所有会话证据链 |
| `/stats` | 内联显示终态分布与 FCR |
| `/sessions` | 打开会话选择器（仅当前工作区），回车恢复并续接该会话 |
| `/diff` `/undo` | 查看工作区 diff / 撤销最近一次自动提交 |
| `/rewind` | 列出检查点；`/rewind restore <id>` 恢复 |
| `/approve <session-id>` | 读取计划模式的 Receipt 并执行该计划 |
| `/context` | 最近一步提示词占用与估算剩余百分比 |
| `/compact` | 说明自动压缩策略（下一回合生效） |
| `/verbose` | 展开/折叠工具输出（等同 Ctrl+O） |
| `/setup` `/help` `/quit` | 配置向导说明 / 帮助 / 退出 |

命令面板：输入 `/` 打开；`↑ / ↓` 选择，`Tab` 补全，`Enter` 执行高亮项。
需要参数的命令（如 `/model`）按 `Enter` 只补全命令名，继续输入参数后再提交。

回合进行中提交的普通任务会进入队列，本回合结束后自动发送；`Esc` 中断并清空队列。
TUI 默认续接同一会话；`/new` 或 `/sessions` 可切换。输入历史落盘到
`%LOCALAPPDATA%\HB\tui_history.json`，下次启动可继续用 `↑ / ↓` 翻。

`/verify` `/stats` `/diff` `/undo` `/rewind` 走注入的 `Exec` 回调；
`/sessions` 走 `ListSessions` / `ResumeSession` 回调。TUI 包本身不依赖 store/engine。

## TUI 快捷键

| 快捷键 | 行为 |
|---|---|
| `Enter` | 提交任务或执行命令；命令面板中执行高亮项 |
| `Shift+Enter / Alt+Enter` | 输入换行（多行任务） |
| `Ctrl+C` | 中断当前回合（空闲时退出）；审批与帮助页同样生效 |
| `Ctrl+L` | 清空对话显示 |
| `Ctrl+O` | 展开/折叠工具输出 |
| `Ctrl+R` | 只读重放当前会话 |
| `Ctrl+Home / Ctrl+End` | 对话区到顶/到底 |
| `Ctrl+U / Ctrl+D` | 对话区上/下半页 |
| `PgUp / PgDn` | 对话区翻页（鼠标滚轮同效） |
| `Esc` | 忙碌时中断并清空队列；空闲时关闭面板/提示 |
| `↑ / ↓` | 翻历史输入 / 在命令面板与会话列表内选择 |
| `Tab` | 补全斜杠命令 |
| `?` | 打开或关闭快捷键帮助 |
| `/` | 打开命令面板 |

## 模型协议

| 协议 | `--protocol` 值 | 端点格式 | 说明 |
|---|---|---|---|
| OpenAI Chat Completions | `openai-chat`（默认） | `{base_url}/chat/completions` | 生态最成熟 |
| Anthropic Messages | `anthropic` | `{base_url}/v1/messages` | 2026 年事实第二标准，Claude Code 生态推动；DeepSeek/MiniMax/xAI/Ollama 亦兼容 |
| OpenAI Responses | `responses` | `{base_url}/responses`（base_url 含 `/v1`） | 语义化 SSE 事件；部分新模型 Responses-only；显式设 `store:false`，会话只存本地 |

自动推断规则：URL 含 `anthropic` 或 `/v1/messages` → Anthropic；URL 以 `/responses` 结尾 → Responses；
其余 → Chat Completions。若 base_url 只含 `/v1`（最常见），Responses 需显式传 `--protocol responses`。
密钥只从环境变量读（默认 `HB_API_KEY`），配置里只存环境变量名。

DeepSeek 思考模式（`deepseek-flash`）默认开启。适配器会解析并回传 `reasoning_content`；
可用 `hb config set reasoning_effort low|high|max` 控制思考强度，编程任务通常 `low` 足够。

## 上下文与撤销

- 每轮自动装配：系统提示 → `HB.md`/`AGENTS.md` → 环境（OS/cwd/日期 + `git status --short` 与 `git diff --stat`）
  → 仓库地图与上一轮历史 → 任务。
- 上下文预算为窗口的 70%；超出时旧对话丢弃、旧工具输出截到 200 字符，并发 `notice{info}` 报告裁剪条数；
  最近 2 轮与 system 永远保留。
- 上下文窗口支持 300k / 600k / 960k 三档，默认 960k：
  `hb config set context.window 600k`；也接受 `300000` 这样的数字。
- 单次输出预算默认 `auto`：`reasoning_effort=max` 时 128k，其余 64k；
  可用 `hb config set max_output_tokens 128k` 固定，`auto` 恢复自动。
- 默认工作区可用 `hb config set workdir D:\code\your-project` 固定；`--cwd` 优先于配置，
  配置优先于进程当前目录。`hb init` 会把选择的工作区写入配置。
- 严格质量契约：`hb config set rigor strict`。它只追加质量提示并检查占位符，
  Gate 仍只认证据；详见 [`rigor.md`](rigor.md)。
- 每轮开始写文件快照检查点（跳过 `.git`/构建产物与 >2MB 文件）；`hb rewind restore` 可把代码恢复到快照，
  对话历史不受影响（对标 Claude Code `/rewind` 与 Aider `/undo`）。
  检查点排除 AppData/OneDrive/包缓存，最多 4000 文件或 128MB；主目录会自动跳过检查点与仓库地图。
- 编辑工具有 `edit_file`（精确替换）与 `apply_patch`（`*** Begin Patch` 格式，支持新增/删除/更新文件）。
  两者的锚点不唯一都报错，不做模糊匹配。