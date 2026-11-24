# v1.0.0 施工路线图

> 目标：按《Honest Boy CLI 全量开发手册 v3》的 §6 完成功能、质量、经济闭合。
> 原则：先闭合已有能力，再加新能力。M7 只有在 M0–M6 全绿后才允许进入 1.0.0；否则进 1.1。
>
> 标记：`[x]` 已完成；`[ ]` 待做。带 `(S/M/L)` 的是相对工作量，不是日历承诺。

## 已完成基线（不回退）

- [x] core 状态机、Provenance、Evidence、Gate、Receipt 哈希链、`hb verify`
- [x] 三套协议：OpenAI Chat、Anthropic Messages、OpenAI Responses
- [x] 沙箱、读写/精确编辑/apply_patch、搜索、shell、build/test/vet 验证器
- [x] 非交互 CLI：run/chat/resume/sessions/verify/stats/probe/config/doctor/init/gc/export/upgrade/undo/diff/approve/rewind/explain
- [x] TUI：实测布局、滚动、队列、审批、命令面板、diff、多行输入、会话恢复
- [x] 多轮续接：`session_id` + `store.Open` + 上一轮历史注入
- [x] v0.1.0 GitHub Release、checksums、winget/scoop 哈希、发布说明
- [x] CI（Linux `-race`、Windows 二进制 e2e、govuln）、依赖方向检查
- [x] 移除 TUI 吉祥物/球场/射门/GOAL，改为固定 3 行标题栏

---

## M0 收尾 v0.1.x（先发补丁）

依赖：无。目标：让已发布的下载版与当前 main 一致，并修掉真实升级阻塞。

- [x] 修 `hb upgrade`：默认 15 分钟、`--timeout` 可配置、TTY 进度、失败重试 3 次 (M)
- [x] 用真实 GitHub Release 完整跑一次 `hb upgrade`，下载/SHA256/替换/版本确认通过 (S)
- [x] 将无吉祥物 TUI + 升级修复发布为 `v0.1.1`，重生成 checksums 并回填 winget/scoop (S)
- [~] winget 清单 `validate` 通过（0.1.0/0.1.1）；本地安装被管理员策略 `LocalManifestFiles` 拦截，scoop 未安装，安装/卸载待干净环境 (M)
- [x] 更新 D05/D06/D13 状态与 `docs/release-checklist.md` (S)

验收：`hb upgrade --check` 与完整升级都通过；包管理器安装可用；`v0.1.1` 资产哈希与清单一致。

---

## M1 TUI 日常交互闭合

依赖：M0。目标：把 TUI 从“能用”做到“连续用一个月不烦”。

- [ ] 大段粘贴折叠为 `[粘贴 N KB]`，提交时还原 (M)
- [ ] `Ctrl+Home` / `Ctrl+End` 到顶/到底 (S)
- [ ] `Ctrl+R` 只读重放当前会话 (M)
- [ ] `Shift+Enter` 换行，同时保留 `Alt+Enter` (S)
- [ ] 普通审批时禁用/置灰输入框；高危审批才允许输入 `yes`；补测试 (M)
- [ ] 高危审批红框；命令超过 6 行可 `Ctrl+O` 展开 (M)
- [ ] 审批等待计时：显示“已等待 Ns” (S)
- [ ] `notice{info}` 3 秒淡出；warn/error 保留到下一次输入或清屏 (M)
- [ ] `/context` 明细：system / HB.md / 历史 / 工具结果 / 本轮；>85% 状态栏变色 (M)
- [ ] `/reload` 手动重载配置；`/theme dark|light|none` (M)
- [ ] `/compact` 真正触发一次压缩，而不只是提示 (M)
- [ ] 统一 `/clear` 与 `/new` 语义并写文档：清显示 / 清上下文 / 新会话 (S)
- [ ] 状态栏补证据数与会话 ID 前 6 位 (S)
- [ ] 决定并落实 `Ctrl+C` 语义：单次不退出、双击退出；或正式修订手册与帮助 (S)
- [ ] `Ctrl+O` 支持按工具块展开，或明确保留全局开关并写清 (M)
- [ ] 首次引导：三步内嵌引导，或明确由 `hb init` 承担并在首屏说明 (M)

验收：手册 T606/T607/T609/T610/T611/T613/T805/T806/T809/T811 的可测项全部通过；TUI 场景手测记录。

---

## M2 富渲染闭合

依赖：M1（布局稳定后再做渲染缓存）。目标：代码、diff、表格在终端里可读。

- [ ] 代码块高亮接入 `render.Highlight` / `FoldCode`；>200 行默认折叠，Ctrl+O 展开 (M)
- [ ] diff 渲染接入行号、3 行上下文、未变段折叠；非 TTY 仍可辨识 (M)
- [ ] 表格渲染接入自适应列宽、中文宽度、单元格换行、数字右对齐 (M)
- [ ] OSC 8 超链接；不支持终端降级为 `文本 (URL)` (S)
- [ ] Markdown 图片显示 `[图片: alt (URL)]`；>200 字符 base64 折叠 (S)
- [ ] golden 快照：80/100/120/200 × dark/light/none，覆盖 40 个样本 (L)
- [ ] 块级 LRU 缓存：hash(content)+width+theme，200 块上限 (M)
- [ ] 200 行代码渲染 <30ms；二次渲染缓存命中 >80% (S)

验收：手册 T701–T712 阶段门；中英混排不错位；`NO_COLOR` 可读。

---

## M3 引擎与工具能力闭合

依赖：M1/M2 可并行，但建议在 M2 后。目标：补齐手册能力矩阵里标为“做”的缺口。

- [ ] `hb run --diff`：把 `git diff` 作为上下文 (M)
- [ ] 并行只读工具：无依赖 read/search/list 并行，写操作串行 (M)
- [ ] 自定义工具 `tools.d/*.json`：命令+schema+Risk+Provenance，显式启用 (L)
- [ ] `web_fetch` 白名单域名，默认关闭，开启需审批 (M)
- [ ] `session_end` 事件；`hb sessions` 显示异常退出标记 (S)
- [ ] 单会话 50MB 轮转到 `session-<id>-2.jsonl` (M)
- [ ] `Registry.Declarations()` 输出真实声明；注册时校验 schema；重名错误区分 (M)
- [ ] 清理 `RunChat`/`RunResume` 死代码，或让入口真正走它们 (S)

验收：手册 T301/T615/T804/T908 与 §6.2 工具矩阵；新增工具都有 schema、Risk、Provenance、测试、文档。

---

## M4 可观测与运维闭合

依赖：M3（工具/网络都稳定后）。目标：失败 5 分钟内定位。

- [ ] `hb run --profile`：HTTP 首字节、流时长、工具耗时、渲染耗时、上下文装配汇总 (M)
- [ ] 上述计时进入 DEBUG 日志与 `usage` 事件 (S)
- [ ] `hb doctor --fix`：重建损坏配置默认值，不动会话 (M)
- [ ] `hb help <topic>`：run/config/evidence/tools/exit-codes (M)
- [ ] 帮助内容 `go:embed` docs，单一真相 (M)
- [ ] `hb upgrade` 每日最多检查一次，结果缓存；`HB_NO_UPDATE_CHECK=1` (S)
- [ ] 配置迁移 `_migrated` 标记并回写 (S)
- [ ] 崩溃文件统一到 HB logs 目录；TUI panic 恢复终端有测试 (M)
- [ ] 脱敏 10 样本验收；日志/崩溃/导出全链路 grep 无密钥 (M)

验收：手册 T901–T910；任意失败回合可从文件还原因果链。

---

## M5 测试与质量硬门

依赖：M1–M4 功能稳定后集中收口，但覆盖率/回归应持续跑。目标：达到 §6.3 数字。

- [ ] `internal/core` 覆盖率从 82.9% 提到 ≥85% (M)
- [ ] `sandbox` / `guard` 分支 100%；12 个逃逸用例三平台通过 (L)
- [ ] Gate 反例 ≥20 个，表驱动 (M)
- [ ] `testdata/sessions` 至少 10 个文件化回放样本，覆盖 done/untested/denied/halt (M)
- [ ] 二进制 e2e 从 3 个补到 5 个：无密钥退出 6、无证据退出 4、中断 130、verify 篡改 5 (M)
- [ ] SSE/Markdown/diff 三个 fuzz 目标各跑 60s，记录无 crash (M)
- [ ] CI 增加 `-count=10` 无 flaky；保持 `-race` (M)
- [ ] 200 回合随机长跑：goroutine 回基线、内存增长 <50MB、无 panic (L)
- [ ] 50 轮 TUI 内存增长 <50MB；空闲 <60MB (M)
- [ ] 性能补测：启动 <300ms、1000 事件恢复 <500ms、10 万文件搜索 <3s (M)
- [ ] 30 个失败场景表 + 7 类极端输入演练记录 (L)
- [ ] `golangci-lint` 进 CI；staticcheck U1000/deadcode 清零 (M)
- [ ] 30 天自用记录从 23 条补到 30 条；每条有修复或不改理由 (L)
- [ ] 3 人陌生测试 ≥2/3 成功；新人 30 分钟跑通记录 (L)

验收：手册 TA01–TA12、TD01–TD07 与 §6.3 全部硬指标。

---

## M6 分发与 v1.0.0 发布

依赖：M5 全绿。目标：干净机器能装、能升级、能卸载、能回滚。

- [ ] Windows amd64/arm64、Linux amd64 三平台真实运行 smoke（不只交叉编译） (M)
- [ ] 代码签名或明确 SmartScreen 说明；README 公布 SHA256 (M)
- [ ] 回滚方案：保留上 2 个版本下载；会话格式升级前 `.bak` (M)
- [ ] `hb doctor --fix` 覆盖损坏配置 (S)
- [ ] 发布 `v1.0.0-rc.1`，跑完整 release checklist (M)
- [ ] 干净 VM：winget/scoop 安装 → 运行 → 升级 → 卸载，记录残留 (L)
- [ ] `v1.0.0` tag、release notes、`docs/frozen-scope.md` 与实际能力一致 (M)

验收：手册 TB01–TB12 与 §6.3.4/§6.5；三项发布门（功能/质量/经济）全绿。

---

## M7 可选增强（只在不拖累 1.0.0 时进入）

规则：M0–M6 未全绿前，以下只进 `v1.1` 分支；不得为了新功能推迟 1.0.0。

- [ ] MCP stdio 客户端：本地 server、只读工具优先、Risk/审批/Provenance (L)
- [ ] 浏览器自动化：CDP + Edge/Chrome；`verify_html` headless smoke；截图辅助 (L)
- [ ] LSP 诊断/引用：接 `gopls` 等，工具输出不自动当 Gate 证据 (L)
- [ ] PostgreSQL/SQLite 只读查询工具：显式 DSN、只读事务、行数上限 (M)
- [ ] 规则式模型路由：只读探索用便宜模型、写/验证用强模型；默认关闭 (M)
- [ ] 视觉截图理解：辅助验收，不替代确定性验证 (M)
- [ ] 多模型投票：显式“高 assurance”模式，成本透明 (M)
- [ ] 向量索引 / 终端图片 / 语音 / 容器：按真实需求后置 (L)
- [ ] 本地 `tools.d` 插件目录（M3 已列，优先于插件市场）(M)

### 1.0.0 明确不做

- 遥测/埋点、云同步、多用户、插件市场、移动端、动画/花哨 UI
- 完整 MCP 的远程/市场形态（先做本地 stdio）
- 自动模型路由的“难度判断”自动决策（只做显式规则）
- 终端图片渲染、语音输入（除非有真实用户阻塞）

---

## 建议执行顺序（单人）

1. M0 收尾 v0.1.x（先修升级超时）
2. M1 TUI 日常交互
3. M2 富渲染
4. M3 引擎/工具
5. M4 可观测
6. M5 质量门
7. M6 v1.0.0
8. M7 只在不影响 1.0.0 时启动

**一句话**：先把 v0.1.x 的升级与 TUI 日用细节闭合，再补渲染和工程能力，最后用测试数字和干净机器安装把 1.0.0 锁死。M7 是加分项，不是 1.0.0 的前置条件。
