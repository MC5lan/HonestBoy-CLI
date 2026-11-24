# 架构

分层：`cmd → cli/tui/engine → core/llm/tools/store`。
TUI 由 `cmd/hb` 装配（注入 engine.Run 回调），`internal/tui` 本身只依赖 core / render / theme，
不直接依赖 llm / tools / store / engine。品牌资源在 `assets/`。
依赖方向由 `scripts/checkdeps.sh`（与 `.ps1`）机器化检查，反向依赖即构建失败。
核心不变量：事件是唯一真相；`terminal` 只能由 Gate 写。

## 界面设计

TUI 采用统一的三段式布局：

- **启动横幅**：`╭──╮` 框线卡片显示版本、模型、目录、提示
- **对话区**：统一用 `•` 前缀；工具动作显示成 `• 读取文件 (12毫秒)`；失败显示 `✗`
- **输入区**：底部固定，`›` 加粗提示符 + 深色背景块
- **状态栏**：底部常驻单行、暗色，格式 `模型 · 目录 · 上下文剩余 N% · 结果`；
  工作时显示 `• 思考中 Ns（esc 中断）`，结束显示 `■ 完成`
- **人类化输出**（CLI 与 TUI 共用 `core` 标签）：工具名/终态/Gate 规则翻译成中文；模型 API 的
  原始 JSON 错误（如 Authentication Fails）转成“密钥不对或没生效 + 三步检查清单”

## 长任务工程能力

- **上下文压缩**：`engine.Budget.Trim` 在主循环每轮调用，system 与最近 2 轮完整保留，旧工具输出压到 200 字符，
  裁剪结果通过 `notice` 事件上报；`WithCompactMark` 明确告知模型历史已被摘要。
- **git 上下文注入**：`EnvContext` 每次回合注入 `git status --short` 与 `git diff --stat`。
- **检查点/撤销**：`CreateCheckpoint` 每轮写工作区文件快照，`hb rewind restore` 逐步恢复。
- **补丁编辑**：`apply_patch` 支持 `*** Begin Patch`，与 `edit_file` 共用读后写校验。
- **仓库符号地图**：`BuildRepoMap` 用标准库 `go/parser` 解析 Go 符号（函数/方法/类型），
  正则补充 Python/JS/Rust，按被引用次数排序，在预算内注入上下文（默认 1200 tokens）；不引入 tree-sitter。
- **git 自动提交与 diff 审阅**：`internal/git` 提供 diff/status/commit/undo；
  回合前的用户脏改动单独快照提交，回合后的写操作自动提交并发 `diff_review` 事件；`hb undo` 混合重置保留改动。
- **计划模式**：`--plan` 时只暴露只读工具，模型产出计划后以 `TermHalt`
  写入 Receipt，`hb approve <id>` 读取计划并正式执行；批准前不会修改任何文件。

## 诚实机制

判定逻辑全部留在 `internal/core`，纯函数、只依赖标准库；UI 层不写判定。

| 机制 | 位置 | 说明 |
|---|---|---|
| 真值分离 | `provenance.go` / `evidence.go` | Provenance 四值 + `CanGround()` |
| 完成权外置 | `gate.go` / `turn.go` | 状态机无 ACT→DONE 边 |
| 形态可测 | `morph.go` | `Analyze` + `EvaluateMorph`（D1/D2/D3） |
| 自评块 | `self.go` | `ParseSelf`；缺少自评先拒执行本步并给两次补救；conf=unknown 或仍缺失 → 阻断写操作并 HALT |
| 歧义暂停 | `ambiguity.go` | A1–A5 可判定谓词 → Pass/Warn/Halt |
| 测试计划三态 | `plan.go` | `PlanItem` + `Finalize` 强制 passed/failed/untested |
| 系统截断 | `plan.go` | `RawOutput` 由系统加截断标记 |
| 权限分级 | `auth.go` | `Authorize`；不可逆单次授权 |
| 轨迹与 UCR | `trace.go` | `ClaimRecord.grounded`；`hb stats` 显示 UCR/FCR |

### 机制接线（engine）

以上谓词不只在 core 里有实现，已接入主循环：

| 机制 | 接线点 | 行为 |
|---|---|---|
| 歧义暂停 | `loop.go` 回合开始 | A1 悬空指代命中 → `TermHalt`，不执行任何工具，写 Receipt |
| 形态观测 | `loop.go` 每步文本 | `core.Analyze` 检出自保证/投降 → `notice{warn}`；自我保证仍触发 Gate 加严（T410） |
| 自评补救 | `loop.go` 每步写操作前 | 缺 `[SELF]` → 拒执行本步并回传缺口，最多两次；明确 unknown 仍 HALT |
| 测试计划 | `loop.go` 解析 + 回合末 `Finalize` | `[TEST PLAN]` 被记录；`passed` 缺可接地证据时告警 |
| 权限分级 | `approval.go` + `loop.go` | `AuthorityOf` 映射风险；`core.Authorize` 判定；`AuthIrreversible` 永不记忆授权 |
| 失败账本 | `engine.go` + `loop.go` | 工具失败追加 `%LOCALAPPDATA%\HB\ledger.jsonl`；下轮按标签/路径检索 Top-3（阈值 0.35）注入上下文，并标明“历史、非本轮观测” |
| 输出预算 | `engine.go` | `MaxOutputTokens` 默认 65536（不超过窗口一半）；`finish_reason=length` 且无工具调用按 failed 结束 |
| 思考模式 | `llm/openai_chat.go` + `engine` | 解析并回传 `reasoning_content`；`reasoning_effort` 可配为 low/high/max |
| 严格质量契约 | `core/rigor.go` + `loop.go` | strict 模式追加完整实现/边界/安全/最小 diff 提示；写入后扫描占位符并回传警告，不改变 Gate |

开关位于 `engine.Config`：`AmbiguityHalt` / `SelfAssessment` / `PlanFirst` / `LedgerRecall`。
CLI `hb run` 与 TUI 都默认开启这四项。

系统提示只陈述结构规则，不做道德劝说：完成仍由 Gate 在代码层强制，不依赖模型“愿意诚实”。

## 设计论文

[`docs/paper/constructive-closure.pdf`](paper/constructive-closure.pdf)（《构造性闭环：面向本地编程智能体的信息溯源隔离与完成门禁》）
是本项目的设计论文。已落地到代码的部分：

- **三条命题**：真值来自验证器而非模型自述；完成权外置到模型不可写的 Gate；坦荡性同时按内容与形态度量。
- **接地谓词**（论文定义 3）：`Evidence.CanGround()` 要求一级验证器 + 实际运行 + 非负退出码 + 通过 + 原文哈希匹配。
- **运行/通过分离**（论文 §4.1）：未运行的验证器按证据缺失处理，不冒充失败或成功。
- **原文哈希**（论文 §4.1）：`Evidence.RawHash` 纳入 Gate 判定与 Receipt 链校验，禁止事后改写摘要。
- **形态观测量**（论文 §5）：跨三条件长度漂移（max/min>1.8 或差>120）、八类自我保证、七类投降表述及双条件消歧，均在 `internal/core/consistency.go`。
- **静态可验证性**（论文 §6.3）：转移表不存在“动作直达完成”的边（`turn_test.go` 全覆盖），接地谓词各反例有单测（`grounding_test.go`）。

## 协议层（internal/llm）

`Client` 是厂商无关的统一接口：`Stream(ctx, Request) (<-chan Chunk, <-chan error)`。
上层只消费 `Chunk`（文本增量 / 完整工具调用 / 用量 / 结束），不感知厂商字段名。

| 适配器 | 文件 | 协议 |
|---|---|---|
| `ChatClient` | `openai_chat.go` | OpenAI Chat Completions（`/chat/completions`） |
| `AnthropicClient` | `anthropic_messages.go` | Anthropic Messages（`/v1/messages`） |
| `ResponsesClient` | `openai_responses.go` | OpenAI Responses（`/responses`） |

`llm.NewClient(protocol, baseURL, apiKey, model)` 是统一工厂；
`llm.DetectProtocol(explicit, baseURL)` 显式配置优先，否则按 URL 推断。

### 协议支持
Anthropic Messages 与 OpenAI Responses 均作为正式协议支持：

- Messages API 已成为广泛使用的第二标准（Claude Code 生态，且 DeepSeek /
  MiniMax / xAI / Ollama 等兼容）；
- Responses API 是 OpenAI 新协议，部分新模型已 Responses-only。

三套协议共用同一 `Client` 接口与 SSE 解析器，协议差异全部隔离在 `internal/llm` 内，
不污染 core / engine / cli。Responses 适配器显式设 `store:false`，避免服务端留存会话，
符合「数据不出机器」定位。
