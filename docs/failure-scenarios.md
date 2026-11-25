# M5 失败场景与极端输入映射（TD04 / TD05）

> 目的：先把**已有失败路径测试**盘清，再建立「场景 → 测试/实测」映射，避免重复造测试。
> 对应手册 TD04（30 个失败场景表，每行有"实际提示"与"是否可操作"）与 TD05（7 类极端输入全部手工跑过并记录）。
> 生成日期：2026-11-24。

## 0. 本次盘点口径与环境限制（先读）

- 盘点范围：`HonestBoy-CLI/` 下 `internal/{core,tools,engine,cli,llm,store,git,errs,tui}`、`e2e/`、`docs/consistency_test.go`。
- 规模：**92 个 `*_test.go`、326 个顶层 `Test*` 函数**。失败路径相关测试集中在 `tools`(8 文件)、`engine`(18)、`cli`(9)、`llm`(8)、`core`(23)、`store`(5)。
- 状态标记：
  - **A** = 已有自动化测试**直接断言**该失败路径的行为/提示；
  - **I** = **间接覆盖**：测试触达底层错误分支，但没有断言面向用户的提示文案；
  - **M** = **需实测**：必须真实环境（磁盘满、只读、权限、断网、超长命令、百万文件），文中给出可复现步骤；
  - **G** = **缺口**：期望行为与现有实现不一致，需先改代码或补测试，不能算"已通过"。
- **验证记录**：Go `go1.27.4 windows/amd64`（`D:\win7harness\go-toolchain\go\bin\go.exe`）已用于复跑：`go test ./... -count=1` **全绿**；新增/修正测试逐条 `-v -count=1` 通过；`HB_E2E=1` 下 **9 条真实二进制 e2e 全过**（含网络断、10MB 单行、只读文件、断流续写）。
- **真实环境实测进度**：F20 断网、F07 只读、E1 10MB 单行、E4 50 万字输出已完成（见 §5.1）。F06 磁盘满（需管理员/VHD）、F08 权限不足、E2 百万文件、E3 10 分钟仍未执行，原因逐项记录在 §5.1。

## 1. 现有失败路径测试盘点（重复造测试之前先看这里）

| 包 / 文件 | 已覆盖的失败路径 | 代表性测试 |
|---|---|---|
| `internal/tools/sandbox_test.go` | 12 个逃逸变体（相对上跳、绝对/盘符/UNC/短名/长路径前缀、URL 编码、NUL）、Open/Create/Stat/Remove/Rename 错误分支、符号链接逃逸 | `TestSandboxEscapeCases`, `TestSandboxMethodErrorBranches`, `TestSandboxSymlinkEscape` |
| `internal/tools/fs_test.go` | 二进制拒绝、分页参数、外部改动后写入被拒、锚点多处/无匹配、目录列表 | `TestReadBinaryAndPagination`, `TestReadThenEdit`, `TestEditAmbiguousAndNoMatch`, `TestListDir` |
| `internal/tools/shell_test.go` | 非零退出码、1s 超时杀进程树、>1MB 输出截断、子进程不继承 `HB_API_KEY` | `TestShellNonZeroExit`, `TestShellTimeout`, `TestTruncateOutput`, `TestFilteredEnvExcludesKey` |
| `internal/tools/apply_patch_test.go` | patch 锚点多处/格式非法 | `TestApplyPatchAmbiguousAndMalformed` |
| `internal/tools/misc_test.go` | 未读先写、读到后变更、工具重名/坏 schema、非法正则、进度条压缩 | `TestGuard`, `TestRegistry`, `TestRegistryRejectsInvalidSchema`, `TestSearchTool` |
| `internal/tools/webfetch_test.go` | 域名白名单、loopback 拒绝、开启 net 但无白名单时注册失败 | `TestWebFetchDomainAllowlist`, `TestWebFetchRejectsLoopback`, `TestRegisterWebFetchRequiresAllowlist` |
| `internal/tools/verify_test.go` | 构建/测试失败、`go test -count=1` 禁缓存、失败名提取 | `TestVerifyBuildAndTest`, `TestVerifyFailingTest`, `TestTestArgsDisablesCache` |
| `internal/tools/custom_test.go` | 自定义工具风险等级非法 | `TestCustomToolRejectsBadRisk` |
| `internal/llm/*_test.go` | HTTP 4xx/5xx、无效 JSON 跳过、超长 SSE 行、回调错误、重试/不可重试分类、context 取消、多字节边界、非法 UTF-8 清洗 | `TestChatStreamHTTPError`, `TestAnthropicHTTPError`, `TestResponsesHTTPError`, `TestChatStreamInvalidJSONSkipped`, `TestParseSSELongLineNoPanic`, `TestWithRetry*`, `TestClassifyStatus`, `TestSanitizeUTF8` |
| `internal/engine/mechanisms_test.go` | 输出被 max_tokens 截断、缺 `[SELF]` 重试/耗尽、歧义 halt、风险警告、上下文压缩 | `TestTruncatedResponseFails`, `TestMissingSelfRetriesThenContinues`, `TestMissingSelfExhausts`, `TestAmbiguityHalt`, `TestCompactionInLoop` |
| `internal/engine/failed_tool_gate_test.go` | 失败工具/失败验证不能走到 done | `TestFailedToolCannotReachDone`, `TestFailedVerifyCannotReachDone` |
| `internal/engine/outside_test.go` | 模型杜撰工作区外路径被拒、用户显式批准后可读并提示 | `TestOutsideAccessRejectsModelInventedPath`, `TestOutsideAccessWithUserApproval` |
| `internal/engine/crash_test.go`, `checkpoint_test.go` | panic 写崩溃日志、检查点 4000 文件/128MB 截断并提示 | `TestCrashRecovery`, `TestCheckpointTruncates`, `TestCheckpointSkipsHeavyDirs` |
| `internal/core/*_test.go` | Gate 六案 + 20 反例、证据篡改、自我保证、歧义、事件解码错误、极端判定输入、封顶截断 | `TestGateSixCases`, `TestGateCounterexampleMatrix`, `TestVerifyChainDetectsEvidenceTamper`, `TestDetectSelfAssurance`, `TestUnmarshalEventErrors`, `TestExtremeInputs` |
| `internal/cli/*_test.go` | 无密钥引导 init、友好错误、升级校验和/坏 JSON/重试/已最新/只检查、退出码、导出脱敏 | `TestRunErrorMentionsInit`, `TestFriendlyErrorAuth`, `TestUpgradeChecksumMismatch`, `TestUpgradeBadReleaseJSON`, `TestUpgradeRetriesTransientAsset`, `TestExitCodeOf`, `TestExportRedacts` |
| `internal/store/*_test.go` | 会话分片滚动、锁竞争、会话不存在、配置迁移 | `TestSessionRotatesAndLoadsAllParts`, `TestSessionLockContention`, `TestConfigRoundTrip`, `TestLoadMigratesMissingFields` |
| `internal/git/git_test.go` | 非 git 仓库、脏工作区、提交/撤销 | `TestNotRepo`, `TestOpenAndDirty`, `TestCommitDiffAndUndo` |
| `internal/errs/errs_test.go`, `docs/consistency_test.go` | 错误码/退出码/命令表/键位表与文档一致 | `TestErrorCodes`, `TestErrorCodesDocumented`, `TestExitCodesDocumented`, `TestCommandTableMatchesHelp` |
| `e2e/e2e_test.go` | 二进制级：无密钥退出 6、无证据退出 4、untested 退出 3、离线命令契约（默认跳过，需 `HB_E2E=1`） | `TestBinaryNoKeyExit`, `TestBinaryDeniedExit`, `TestBinaryUntestedExit`, `TestBinaryOfflineCommands` |
| `internal/tui/*` | 审批流、Ctrl+C/Esc、命令面板、主题、高风险长命令展开、20+ 回归项 | `TestApprovalFlow`, `TestCtrlCInterruptsPendingApproval`, `TestHighRiskApprovalReturnsModel` 等 |

**结论**：工具层、Gate/证据层、协议层、CLI 升级与退出码的失败路径**已大量自动化覆盖**，TD04 的 30 条应优先复用上表，只补"真实环境类"和"缺口类"。

## 2. TD04：30 条失败场景 → 测试/实测映射

"实际提示"取自源码常量或测试断言；"可操作"按手册要求判断是否给出了下一步动作。

| # | 失败场景（人为制造方式） | 现有测试 / 实测映射 | 实际提示 | 可操作 | 状态 |
|---|---|---|---|---|---|
| F01 | `read_file` 路径不存在（删掉文件或给不存在路径） | `TestSandboxMethodErrorBranches`(Open(missing))、`TestSandboxFileRoundTrip`(删除后 Open) | `读取失败: <os err>`（`fs.go:97`） | 部分：未带路径与建议 | **I** |
| F02 | `read_file` 沙箱越界：`../../`、绝对路径、盘符相对、UNC、短名、`\\?\` 长路径、URL 编码、NUL | `TestSandboxEscapeCases`（12 变体，Windows 全开） | `HB-SBX-003`：把外部文件复制进工作区；确需读取则在输入中显式给出路径并批准 | 是 | **A** |
| F03 | 符号链接指向沙箱外文件 | `TestSandboxSymlinkEscape`（Windows 跳过，需实测） | `os.Root` 拒绝打开 | 部分：无专用文案 | **M** |
| F04 | `read_file` 读二进制（含 NUL 字节） | `TestReadBinaryAndPagination` | `binary file`（`ErrBinaryFile`） | 弱：英文哨兵，未提示用何种工具 | **A** |
| F05 | `read_file` 超过 100MB 文件（稀疏文件即可） | `TestReadFileTooLarge`（稀疏文件断言读取前拒绝） | `file too large` | 部分：未提示"用 offset/limit 分块" | **A** |
| F06 | 写入时磁盘满（小分区/quota 写满） | 无测试 | 目前走通用 `读取/写入失败: <err>` | 部分：需给出剩余空间检查建议 | **M** |
| F07 | 写入只读文件（`attrib +R` / `chmod 444`） | `TestBinaryReadOnlyFile`（真实二进制 + 只读属性，已实跑） | `原子替换失败（HB-IO-001：检查路径权限、只读属性与剩余空间）: … Access is denied.` | 是 | **A** |
| F08 | 权限不足（对临时目录 `icacls` 拒绝写入） | `TestSandboxMethodErrorBranches`（错误分支，非真实 ACL） | `HB-IO-001`：检查路径权限与剩余空间 | 是（错误码层） | **M** |
| F09 | `write_file`/`edit_file` 未读先写 | `TestGuard` | `please read the file before writing` | 是：先 `read_file` | **A** |
| F10 | 读取后文件被外部改动再写 | `TestGuard`、`TestReadThenEdit` | `file changed since last read, please read again` | 是：重新读取 | **A** |
| F11 | `edit_file`/`apply_patch` 锚点不唯一或不存在 | `TestEditAmbiguousAndNoMatch`、`TestApplyPatchAmbiguousAndMalformed` | `匹配到 N 处` / `未找到匹配` / 缺少 `Begin Patch` | 是：给出唯一上下文或修正格式 | **A** |
| F12 | `list_dir` 路径不存在 | `TestListDirMissingPath`（路径不存在 + 沙箱越界均断言失败提示） | `目录不存在或无法访问: …。先用 list_dir . 查看当前目录。` / `路径越界: …` | 是 | **A** |
| F13 | `search` 非法正则 | `TestSearchTool` | 工具失败结果 `OK=false` | 部分：未提示检查正则语法 | **A** |
| F14 | `shell` 命令非零退出（含退出码） | `TestShellNonZeroExit` | `OK=false` + `meta.exit_code`，system prompt 要求附原文输出与退出码 | 是：按原文输出修正命令 | **A** |
| F15 | `shell` 超时（1s 跑 `sleep 30`/`ping -n 30`） | `TestShellTimeout`（新增断言提示含 `-v` 与 `timeout_ms`） | `command timeout: 命令超时；可给命令加 -v 输出进度，或用 timeout_ms 延长超时` | 是 | **A** |
| F16 | `shell` 输出 >1MB | `TestTruncateOutput` | `[... 省略 N 字节 ...]`（保留头 200 + 尾 100 行） | 是 | **A** |
| F17 | `shell` 输出含非法 UTF-8 | `SanitizeOutput` 实现；`TestSanitizeUTF8` 验证的是 llm 侧同名逻辑 | 非法字节被替换为有效字符 | 是（无崩溃） | **I** |
| F18 | 子进程继承密钥 | `TestFilteredEnvExcludesKey` | 环境变量白名单，不含 `HB_API_KEY` | 是（预防性） | **A** |
| F19 | `web_fetch` 非白名单域名 / loopback | `TestWebFetchDomainAllowlist`、`TestWebFetchRejectsLoopback`、`TestRegisterWebFetchRequiresAllowlist` | 拒绝并提示配置白名单 | 是 | **A** |
| F20 | 模型端点断网 / 连接被拒（`--base-url http://127.0.0.1:9`） | `TestChatStreamHTTPError`、`TestAnthropicHTTPError`、`TestResponsesHTTPError`、`TestBinaryNetworkDownMessage`（真实二进制，已实跑） | `HB-NET-001`：检查 base_url / HTTPS_PROXY / 防火墙；`hb doctor`；退出 6 | 是 | **A** |
| F21 | 请求/流式超时 | `TestExitCodeOf`(ErrTimeout→7)、`FriendlyError` 有超时文案但无直接断言、`HB-NET-002` 仅文档 | `HB-NET-002` / "操作超时了。可以加 `--timeout 10m` 再试。" | 是（文案未测） | **I** |
| F22 | 429 / 5xx 瞬态失败 | `TestWithRetrySucceedsAfterFailures`、`TestUpgradeRetriesTransientAsset` | 自动重试 3 次后成功或报错 | 是 | **A** |
| F23 | 认证失败（密钥错误） | `TestFriendlyErrorAuth` | "密钥不对"（不暴露原始 JSON） | 是：检查环境变量 | **A** |
| F24 | SSE 非法 JSON / 超长行 / 回调错误 | `TestChatStreamInvalidJSONSkipped`、`TestParseSSELongLineNoPanic`、`TestParseSSECallbackError` | 跳过非法块；超长行报错；不 panic | 部分 | **A** |
| F25 | SSE 中途断连 | `TestChatDisconnectNoPanic`（`DisconnectAt=1`）、`TestEngineResumeAfterDisconnect`、`TestBinaryResumeAfterDisconnect` | 适配器发 `Disconnected`，引擎注入 `ResumePrompt` 并在同回合续写（最多 2 次），超限明确失败 | 是 | **A** |
| F26 | 无证据申请 `done` | `TestEngineNoEvidenceDenied`、`TestGateSixCases`、`TestBinaryDeniedExit`(exit 4) | `HB-GATE-002`：按 `gate_deny` 的 Missing/Reason 补证据 | 是 | **A** |
| F27 | 证据链被篡改 | `TestVerifyChainDetectsEvidenceTamper`、`hb verify`(exit 5) | "证据记录被改动过，校验没通过。用 `hb verify` 查看详情。" | 是 | **A** |
| F28 | 工具失败 / 验证失败后想完成 | `TestFailedToolCannotReachDone`、`TestFailedVerifyCannotReachDone` | Gate 拒绝完成，先修失败项 | 是 | **A** |
| F29 | 纯文本回答、自我保证、弱证据 | `TestEngineTextOnlyUntested`、`TestBinaryUntestedExit`(exit 3)、`TestDetectSelfAssurance`、`TestEngineSelfAssuranceTightensGate` | "没验证就停下了（这是合法的诚实结果）"；自我保证使 Gate 加严 | 是 | **A** |
| F30 | 无密钥 / 配置无效 | `TestRunErrorMentionsInit`、`TestBinaryNoKeyExit`(exit 6)；配置无效仅 `HB-CFG-001` 文档/错误码层，无直接测试 | "还没准备好…" / 指引 `hb init`；"用 `hb config list` 检查" | 是（无密钥部分已测） | **A** |

### 额外已覆盖的命令级失败路径（不计入 30 条，避免重复造测试）

升级校验和不匹配 `TestUpgradeChecksumMismatch`、release JSON 损坏 `TestUpgradeBadReleaseJSON`、瞬态失败重试 `TestUpgradeRetriesTransientAsset`、升级占用文件先改名 `.old`（dogfood #2）、会话锁竞争 `TestSessionLockContention`、跨目录恢复以记录工作区为准（dogfood #20）、panic 写崩溃日志 `TestCrashRecovery`、检查点超限提示 `TestCheckpointTruncates`、上下文超预算裁剪 `TestBudgetTrimKeepsSystemRecentAndToolSummary`、计划模式过滤写工具 `TestPlanModeFiltersWriteTools`、非 git 仓库 `TestNotRepo`、文档/代码漂移 `docs/consistency_test.go` 四测。

## 3. TD05：7 类极端输入 → 测试/实测映射

| # | 输入 | 期望 | 现有测试 / 实测映射 | 源码实际行为 | 状态 |
|---|---|---|---|---|---|
| E1 | 10MB 单行文件 | **拒绝并提示** | `TestReadLineTooLong` + `TestBinaryHugeSingleLine`（真实 10MB 文件，已实跑） | `read_file` 单行超过 `maxSingleLineSize=1MB` 直接拒绝，正常长行仍 truncates 为 `[line truncated]` | **A** |
| E2 | 100 万文件目录 | 截断并提示缩小范围 | `TestCheckpointTruncates`（4000 文件/128MB 上限 + notice）、新增 `TestListDirTruncatesAt500`（501 文件断言提示）；百万级目录仍需实测 | 检查点与 `list_dir` 均有截断并提示缩小范围 | **A** + **M** |
| E3 | 10 分钟无输出命令 | 超时提示 + 建议加 `-v` | `TestShellTimeout`（1s 超时 + 新增断言 `-v`/`timeout_ms` 提示）；10min 真实场景需实测 | `command timeout: …可给命令加 -v 输出进度，或用 timeout_ms 延长超时` | **A** + **M** |
| E4 | 模型输出 50 万字 | 流式正常、内存不爆 | `TestExtremeInputs`（1000 事件 × 300 字）、`TestMaxOutputTokensBudget`、`TestTruncatedResponseFails`、`TestBudgetTrimKeepsSystemRecentAndToolSummary`、`TestFiftyTurnsMemoryBounded`；**实测** 100 片 × 5000 字：678ms、峰值 34–41MB | 正常分片流式完成；单条 SSE `data:` 行超过 1MB 会被 `ErrSSELineTooLong` 拒绝（有测试），这是安全上限而非崩溃 | **A** |
| E5 | 模型输出全是乱码 | 不崩、可中断 | `TestChatStreamInvalidUTF8`（`testserver.InvalidUTF8` 已实装 `%BAD%` 注入）、`TestEngineInvalidUTF8OutputNoPanic`、`TestEngineInvalidUTF8CanBeInterrupted`（挂起后 cancel）、原有 `TestSanitizeUTF8` | 适配器 + 引擎事件边界双重 `SanitizeUTF8`；事件流恒为合法 UTF-8，可中断 | **A** |
| E6 | 用户输入只有 emoji | 正常处理 | `TestEngineEmojiOnlyPrompt`（引擎级：输入完整保留、终态正常、事件无非法 UTF-8）+ 原有 `TestExtremeInputs` | emoji 输入按普通任务处理，不 panic | **A** |
| E7 | 会话文件被截断 | 恢复到最后一个完整行 | `TestLoadRecoversTruncatedTail`（末行半 JSON 被跳过、事件数正确、`RecoveredTail=true`）+ `TestLoadRejectsCorruptMiddleLine`（中间损坏仍报错） | `SessionStore.Load` 只跳过**末片末行**的外层 JSON 损坏；其它位置损坏照常报错，避免误判 | **A** |

## 4. 缺口修复记录（G1–G7 已完成并实跑验证）

| ID | 缺口 | 已做修改 | 验证测试 |
|---|---|---|---|
| G1 ✅ | E7 会话截断不恢复 | `SessionStore.Load` 用"暂存上一行"方式识别末片末行；仅外层 JSON 解析失败的尾行被跳过并置 `RecoveredTail`，中间/其它分片损坏仍报错 | `TestLoadRecoversTruncatedTail`, `TestLoadRejectsCorruptMiddleLine` |
| G2 ✅ | E1 10MB 单行未拒绝 | `read_file` 新增 `maxSingleLineSize=1MB` 与 `ErrLineTooLong`，超限拒绝并提示用 shell 切分；普通长行仍截断 | `TestReadLineTooLong` |
| G3 ✅ | E3 超时无 `-v` 建议 | `ErrCommandTimeout` 文案改为"可给命令加 -v 输出进度，或用 timeout_ms 延长超时" | `TestShellTimeout`（新增断言） |
| G4 ✅ | E5/E6 缺引擎级断言 | `testserver.InvalidUTF8` 实装（`%BAD%`→非法字节）；引擎在事件边界二次 `SanitizeUTF8` | `TestChatStreamInvalidUTF8`, `TestEngineInvalidUTF8OutputNoPanic`, `TestEngineInvalidUTF8CanBeInterrupted`, `TestEngineEmojiOnlyPrompt` |
| G5 ✅ | `list_dir` 500 条截断无测试 | 补 501 文件测试 | `TestListDirTruncatesAt500` |
| G6 ✅ | `list_dir` 路径不存在静默空 | `ListDirTool.Run` 先做 `sb.Rel` + `os.Stat` + `IsDir` 校验并返回可执行提示 | `TestListDirMissingPath` |
| G7 ✅ | F05/F25 无测试 | F05 稀疏文件断言读取前拒绝；F25 用 `DisconnectAt=1` 断言断流不崩不报错 | `TestReadFileTooLarge`, `TestChatDisconnectNoPanic` |

**后续增强（已部分完成）**：
- ✅ `llm.ResumePrompt` 已接入：三个适配器在缺少结束标志时发 `Disconnected`，引擎同回合续写，最多 2 次（F25）。
- ✅ 会话级续跑第一版：`hb resume --last` 检测强杀/超时/网络/失败会话，注入原始任务与已完成工具，最多 2 次；用户中断不恢复（详见 `docs/commands.md`）。
- ⏳ 自动续跑（无需用户执行命令）尚未默认开启；按业界经验应先 opt-in，跑稳后再考虑。
- **G8 真实环境类 M 项**：磁盘满/只读/权限/断网/百万文件/10 分钟/50 万字，按第 5 节在真实机器执行并记录，不能伪造通过。

## 5. 需实测项目的可复现步骤（带 Go 的机器上执行）

> 约定：以下命令在 `HonestBoy-CLI/` 下执行；用 `$env:HB_E2E="1"; go test ./e2e -v -count=1` 跑二进制级测试。
> 所有会留下痕迹的操作都在临时目录进行，结束删除。

1. **F06 磁盘满**：管理员 PowerShell 用 `diskpart` 创建 8MB VHD 并挂载到空盘符 → `hb run --cwd X:\work "写一个 100MB 文件"` → 记录是否给出"检查剩余空间"提示；`detach vhd` 清理。无管理员权限时如实记"未实测"。
2. **F07 只读文件**：`Set-ItemProperty -Path .\tmp\a.txt -Name IsReadOnly $true` → 让模型 `write_file` 到该文件 → 记录提示，随后恢复属性。
3. **F08 权限不足**：对临时目录 `icacls .\tmp /deny "$env:USERNAME:(M)"` → 写入 → 记录提示 → `icacls .\tmp /grant "$env:USERNAME:(M)"` 恢复。**不要对真实项目目录操作**。
4. **F20 断网**：`hb run --base-url http://127.0.0.1:9 "你好"` → 期望退出 6 + `HB-NET-001` + `hb doctor` 建议。
5. **E2 百万文件**：在 `%TEMP%\hb-1m` 下生成 1,000,000 个空文件（需数分钟）→ `hb run --cwd %TEMP%\hb-1m "列出目录文件"` → 记录检查点 notice 与 `list_dir` 截断提示 → 删除目录。
6. **E3 十分钟无输出**：让模型调用 `shell` 执行 `ping -n 600 127.0.0.1 > $null`（`timeout_ms=600000`）并用 `hb run --timeout 11m` → 观察 10 分钟后是否超时退出；记录是否有 `-v` 建议 → 结束残留进程。
7. **E4 50 万字**：用本地脚本化模型服务返回 50 万字 SSE 流 → `hb run` 期间用任务管理器/`Get-Process` 记录内存峰值 → 期望流式不卡、内联结束、内存回落。
8. **E7 截断会话**：复制一条真实 `.jsonl`，用 `Get-Content` 截取到某行中间另存 → `hb resume <id>` → 期望恢复到最后一个完整行（当前实现会直接报 `bad jsonl line`，属 G1，修完再跑）。

### 5.1 实测结果（2026-11-25，本机 Windows amd64）

| 场景 | 执行方式 | 实测结果 | 结论 |
|---|---|---|---|
| F20 断网 | 临时配置 `base_url=http://127.0.0.1:9`，真实 `hb run` + e2e `TestBinaryNetworkDownMessage` | 退出码 **6**；stdout/stderr 均为"模型端点无法连接。检查 base_url、HTTPS_PROXY 与防火墙；用 hb doctor 自检。"；裸 `dial tcp` 不再泄漏 | ✅ 通过 |
| E1 10MB 单行 | 生成 10MB 单行文件，模型调 `read_file`；e2e `TestBinaryHugeSingleLine` | 工具返回 `line too long: 单行超过 1MB，已拒绝读取；请先用 shell 把它切分成正常换行的文件…`；终态 `untested`（合法） | ✅ 通过 |
| F07 只读文件 | 临时工作区 `chmod 0400` 后模型先读再写；e2e `TestBinaryReadOnlyFile` | 工具返回 `原子替换失败（HB-IO-001：检查路径权限、只读属性与剩余空间）: renameat … Access is denied.`；终态 `untested`，不崩溃 | ✅ 通过（提示已可操作） |
| F06 磁盘满 | 需管理员创建 8MB VHD 挂载 | 当前会话**非管理员**，未能制造真实磁盘满 | ⏸ 未执行，不伪造 |
| F08 权限不足 | 需对临时目录设置 deny ACL | 未执行（避免 ACL 误伤；可在隔离 VM 按 §5 步骤补测） | ⏸ 未执行 |
| E2 100 万文件 | 生成 1,000,000 空文件后 `hb run` | 未执行（生成耗时数分钟且占大量 inode；已用 501 文件单测验证截断逻辑） | ⏸ 未执行 |
| E3 10 分钟无输出 | `timeout_ms=600000` + `hb run --timeout 11m` | 未执行（需真实等待 10 分钟；1s 超时与提示已由 `TestShellTimeout` 覆盖） | ⏸ 未执行 |
| E4 50 万字输出 | 本地脚本化 SSE：100 片 × 5000 字，真实 `hb run`，每 30ms 采样 WorkingSet | 678ms 完成；内存峰值 **34–41MB**；终态 `untested`（纯文本合法），无崩溃 | ✅ 通过（内存未爆） |
| E7 会话截断 | 复制真实 jsonl 并截断末行 | 由 `TestLoadRecoversTruncatedTail` 覆盖；e2e 未单独跑 | ✅ 自动化通过 |

**补充修复（实测发现）**：原 `FriendlyError` 对连接失败只输出裸 `dial tcp` 文本；现已保留 `*llm.RetryableError` 原因并映射到 `HB-NET-001` 建议，`--debug` 时才打印原始错误。新增 `TestFriendlyErrorNetwork`、`TestFriendlyErrorDialTCP` 与 `TestBinaryNetworkDownMessage`。

## 6. 结论与后续

- **修复后 30 条 TD04 状态**：**A 24 条 / I 3 条 / M 3 条 / G 0 条**；G1–G7 全部关闭，F07 已完成真实二进制实测。M 项为 F03（符号链接 Windows）、F06（磁盘满）、F08（权限不足）。
- **修复后 7 类 TD05 状态**：E1、E4、E5、E6、E7 已全部 **A**（E4 实测峰值 34–41MB）；E3 为 **A+M**（超时与提示已测，10 分钟真实等待待测）；E2 为 **A+M**（截断逻辑已测，百万文件目录待测）。
- **验证**：`go test ./... -count=1` 全绿（Go 1.27.4 / windows-amd64）；12 条新增或修正测试单独 `-v -count=1` 通过。
- **断流续写已接入**：适配器在缺少 `[DONE]`/`message_stop`/`response.completed` 时发 `Chunk{Disconnected}`；引擎追加已输出内容 + `ResumePrompt` 后同回合续写，最多 2 次，超限明确失败（`TestEngineResumeAfterDisconnect`、`TestEngineResumeRetriesExhausted`）。
- **仍待办**：F06 磁盘满（需管理员）、F08 权限不足、E2 百万文件、E3 10 分钟真实等待（原因见 §5.1）。完成后才把 `roadmap-v1.0.md` 对应项从 `[~]` 改为 `[x]`。
