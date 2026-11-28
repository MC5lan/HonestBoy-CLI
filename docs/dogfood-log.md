# 自用记录

TD06 要求累计 30 条真实使用中的“今天哪不舒服”。当前是开发期记录，只写实际发生过的问题，
不补写、不编造；正式发布后按使用频率继续累计。

## 2026-11-24

18. TUI 交互专项排查（临时副本探针 + 回归测试），发现并修复：
    - 高危审批按 `y/a/n/esc` 时 `Update` 返回空 `tea.Model`，下一帧 `View()` 空指针，TUI 直接退出。
    - `outside_read` 的 `Command` 是路径、`HighRisk=true`，卡片却用 `IsHighRiskCommand(Command)` 反推成普通审批；
      卡片写 `[y] 允许`，按键却要求 `yes`。现在卡片和按键统一用请求里的 `HighRisk`。
    - 80×24 / 100×30 出现审批卡时，选项行与输入框被裁掉；布局改为实测各区块高度。
    - 命令面板回车执行输入框原文（选中 `/config` 却报“未知命令 /”）；选中第 7 项后高亮滚出窗口。
    - 回合未结束时回车会并发启动第二个 `Run`；现在排队并在结束后自动发送。
    - 审批时 Ctrl+C 被输入框吞掉；帮助页 Ctrl+C 也无响应。
    - PgUp/滚轮无法回看历史；成功的工具输出折叠后没有任何键能展开。
    - `theme=none` 下横幅/审批/Markdown 仍输出颜色码；流式 Markdown 每次刷新重渲整段。
    - 首次 Gate 通过后射门动画冻结；上一回合的 `done/GOAL` 带进新回合。
    - “上下文剩余”用本回合 `In+Out` 累加和，估成“上下文占用”；改用最近一步提示词。
19. 多轮上下文实际不存在：引擎每回合新建 `core.Session`，`messages` 只有 system + 本轮输入。
    现在 TUI 传 `session_id`，`store.Open` 续接会话，`historyText` 注入上一轮历史；Receipt 链继续追加。
20. `hb resume` 从其他目录恢复时以会话记录的工作区为准，避免沙箱与历史错位；`/sessions` 仅列当前工作区会话。
21. v0.1.0 发布演练：CI 三个 job 全绿（含 Linux `-race`）；ConPTY 真实 TUI 的启动/帮助/命令面板/`/config`/Ctrl+C 通过；
    真实模型在临时 Go 仓库完成“修 Add → test/build/vet → Gate `done`”，10.8 秒、退出码 0；
    `goreleaser --skip=publish` 产物 SHA256 全部匹配，本地 `hb upgrade` 成功升级到 `0.1.0`。
22. v0.1.0 正式发布：tag 与 GitHub Release 已推送，3 个 zip + `checksums.txt` 上传；
    线上 checksums 与本地一致（windows/amd64 `015b4968…`，windows/arm64 `324231c1…`，linux/amd64 `c04fc826…`）；
    winget/scoop 哈希已回填。真实 `hb upgrade --check` 通过；但本机到 GitHub 资产链路约 19KB/s，
    6.8MB 下载超过 2 分钟超时，完整 `upgrade` 未能完成——已记入 D13，待改超时/重试后在正常网络复测。
23. TUI 视觉简化：移除吉祥物、像素球场、颠球/射门动画和 `GOAL!`，顶部固定为 3 行标题栏；
    删除了 `pitch.go` / `pixel.go` / `pitch_tick.go` 及动画状态，对话区高度增加 10 行（大终端）。
24. `hb upgrade` 真实网络问题：v0.1.0 的 2 分钟下载超时在约 19KB/s 链路上必失败。
    默认改为 15 分钟、加 `--timeout`、网络/429/5xx 重试 3 次、TTY 进度；
    重新对真实 GitHub v0.1.0 Release 跑完整升级，下载→SHA256→替换→版本确认全部通过。
25. `v0.1.1` 发布：包含无吉祥物 TUI 与升级修复；线上三个 zip + checksums 核对通过，
    winget/scoop 更新到 0.1.1；用修复后的二进制真实升级到 `hb 0.1.1` 成功。
26. TUI 日用小交互补齐：大段粘贴折叠/还原、Ctrl+Home/End、Ctrl+R 只读重放、Shift+Enter、
    普通审批输入禁用、高危红框/等待计时、状态栏证据数与会话 ID。以上均有回归测试。
27. TUI 第二批：info 通知 3 秒淡出、/theme、/reload、真正的 /compact、/reset、
    Ctrl+C 双击退出、Ctrl+O 最后工具块/审批长命令展开。
28. M1 真机验收（ConPTY 驱动真实 hb chat）：启动、帮助、命令面板、/config、/theme、/reload、
    /compact、无会话时 Ctrl+R 提示、Ctrl+C 首按提示/二按退出全部通过。
29. `/context` 已接入 engine 的分项近似 token：system / HB.md / 历史 / 工具结果；
    低于 15% 时状态栏转警告色。
30. M2 富渲染：代码块行号/200 行折叠、diff 行号、图片/base64 降级、OSC8、
    40 个 golden 快照、200 块 LRU 缓存。200 行代码实测 35.5ms，离 30ms 目标还差一点。
31. M3 引擎/工具：`--diff`、`tools.d` 自定义工具、`web_fetch` 白名单、`session_end`
    与异常退出标记、注册表 schema/声明校验；删除 `RunChat`/`RunResume` 死代码。
32. M3 收尾：同一模型步只读工具并行；会话超过 50MB 自动分片，
    Load/List/Delete/AllReceipts 合并分片并有回归测试。
33. M4 可观测：`--profile` 耗时汇总、`doctor --fix`、`help <topic>`、
    update 24h 缓存/关闭开关、崩溃日志移入 HB logs。`go:embed` 帮助、
    `_migrated` 配置迁移与 10 样本脱敏审计仍未做。
34. M5 开局：core 覆盖率 82.9%→95.3%，补 labels/ledger/evidence/session 测试；
    e2e 增至 5 个；core/tools/tui 的 `-count=10` 本地通过。
35. M5 测试硬化：沙箱逃逸样本补到 12 个；SSE/Markdown/diff 三个 fuzz 各 60s 本地通过。
36. M5 回放/长跑：12 个文件化会话样本；Gate 反例 20 个；200 回合连续回合无 panic。
    sandbox 文件操作错误分支覆盖到 100%（Open 75%）；TUI 内存阈值、30 天自用/陌生人测试仍未完成。
37. M4 收尾：配置缺失字段自动回写 `_migrated`；脱敏样本扩到 10 个并补 `sk_live_`/`sk_test_`。
38. M5 性能：`hb --version` 57–64ms；1000 事件恢复 9.8ms；50 轮 TUI 堆增长 <50MB。
    10 万文件搜索、30 失败场景/7 极端输入、golangci/deadcode 与陌生人测试仍未完成。
39. M5 TD04/TD05 静态盘点：新增 `docs/failure-scenarios.md`，盘点 92 个测试文件/326 个测试函数，
    建立 30 条失败场景与 7 类极端输入的「场景 → 测试/实测」映射，避免重复造测试。发现 7 个缺口：
    ① 会话文件截断时 `Load` 直接报 `bad jsonl line`，不恢复到最后完整行；
    ② 10MB 单行文件是截断而非"拒绝并提示"；③ shell 超时无 `-v` 建议；
    ④ 乱码/emoji 缺引擎级端到端断言；⑤ `list_dir` 500 条截断无测试；
    ⑥ `list_dir` 路径不存在静默返回空列表；⑦ SSE 断连无测试（`DisconnectAt` 字段闲置）。
    当时未找到 Go 工具链（随后在 #40 定位到 `D:\win7harness\go-toolchain` 并复跑），
    磁盘满/只读/权限/断网/10 分钟/百万文件仍需实机演练。
40. M5 TD04/TD05 缺口修复（G1–G7）：会话 jsonl 末行截断只跳过末片尾行并置 `RecoveredTail`，中间损坏仍报错；
    `read_file` 单行 >1MB 改为拒绝并提示；shell 超时提示补 `-v`/`timeout_ms`；`testserver.InvalidUTF8` 实装，
    引擎在事件边界二次净化非法 UTF-8；`list_dir` 路径不存在/越界改为可执行提示、500 条截断补测试；
    >100MB 文件与 SSE 断连补测试。找到 Go 1.27.4（D:\win7harness\go-toolchain），
    `go test ./... -count=1` 全绿，12 条新增/修正测试逐条通过；真实环境项仍未演练。
41. M5 断流续写与实测：三个适配器在缺少结束标志时发 `Disconnected`，引擎追加已输出内容 + `ResumePrompt`
    后同回合续写，最多 2 次；真实二进制实测断网（退出 6，提示 base_url/HTTPS_PROXY/hb doctor，不再泄漏裸 dial 文本）、
    10MB 单行（拒绝并提示）、只读文件（`HB-IO-001` 建议置顶，避免被 70 字截断）。
    50 万字输出实测：100 片 × 5000 字，678ms、内存峰值 34–41MB，终态 untested 不崩溃；
    断流续写真实二进制 e2e 通过（两段拼接 + "连接中断"提示 + 2 次请求）。
    磁盘满需管理员、权限 ACL/百万文件/10 分钟未执行，已在 `failure-scenarios.md` 记录原因。
42. 质量契约改常开：默认 `rigor=strict`，旧配置的 `normal` 在 Load 时收敛为 strict，
    `RigorEnabled` 恒为 true（normal/空/未知值都不能关闭），`hb config set rigor normal` 明确报错，
    `hb config list/get` 显示 `strict（常开）`。新增 `TestRigorAlwaysOn` 验证旧配置下仍注入质量契约。
43. v0.2.0 发布：提交 `71bedc9`，CI run 45448821770 三个 job（`-race` build / e2e-windows / govuln）全绿；
    tag 与 GitHub Release 已推送，三个 zip + `checksums.txt` 上传成功。本机覆盖安装 0.2.0
    （旧二进制备份为 `hb.exe.0.1.0-dev.bak`），`hb probe` 三态通过，`hb upgrade --check` 确认最新；
    winget 0.2.0 清单与 scoop `hb.json` 已回填真实 SHA256。goreleaser 本机下载超时，
    产物用相同 ldflags 手工交叉编译 + zip + checksums 构建。
44. 会话级续跑第一版：`hb resume --last` 检测当前工作区最近一条未完成会话（强杀/超时/网络/失败），
    把原始任务、已完成/失败工具和上次输出注入上下文；已完成工具不自动重放；用户 Ctrl+C/untested/denied/done 不恢复；
    同一会话最多续跑 2 次，`turn_start`/`turn_end` 记录续跑次数与原因，`hb sessions` 显示可恢复标记。
    新增 store/engine/e2e 测试；全量 `go test ./...` 与 11 条二进制 e2e 通过。
45. 可读输出修正：用户反馈“基本见不到有用的文字信息”。定位到两个原因：
    `[SELF]`/`[TEST PLAN]` 控制块被原样当正文打印；`request_done.summary` 只写进 Receipt、CLI 只显示“检查通过”。
    现在 CLI/TUI 过滤控制块并保留正常正文，历史注入与续跑提示同样只看正文；GateAllow/TurnEnd 携带并显示最终摘要；
    系统提示要求模型在 request_done 前用 1–3 句话说明做了什么/验证了什么/还有什么没做。
    另加 `hb run --auto-resume`：无新任务时自动续跑最近未完成会话。
46. v0.3.0 发布：会话级续跑、`--auto-resume`、可读输出修正进入正式版；
    tag/Release 已推送，三个 zip + checksums 上传，winget/scoop 0.3.0 回填真实 SHA256；
    本机覆盖安装 0.3.0（旧 0.2.0 备份为 `hb.exe.0.2.0.bak`），`hb probe` 与 `hb upgrade --check` 验证通过。
    winget 新包 PR #488872 已提交（个人项目 CLA 已签）；01–07 校验通过，
    08 安装验证在 Microsoft 外部队列等待。
47. winget 安装验证卡住数小时，定位到 `hb` 在“无参数 + 无配置 + 无 TTY”时会启动设置向导并等待 stdin，
    校验器的安装后探测因此挂起。修复为先判断 TTY（无 TTY 直接打印帮助），新增 `TestBinaryNoArgsNonTTY`；
    发布 v0.3.1；旧 PR #488872（0.3.0）关闭，新 PR #489065（0.3.1）校验通过
    （`Validation-Completed` / `Azure-Pipeline-Passed`），等待 winget-pkgs moderator 合并；
    本机安装 0.3.1 验证通过。
48. 首次体验第一批：`hb` 启动时检测当前工作区最近一条未完成会话并在 TTY 下提示
    “回车继续 / n 新建 / s 查看”；`hb doctor` 重做为用户检查（配置/模型/密钥/端点/工作区/
    会话目录/终端/ripgrep + 建议下一步，HEAD 失败回退 GET）；`hb init` 增加可选真实连接测试；
    README/故障排查补齐 release 下载、SHA256、SmartScreen 与首次运行顺序。
    新增 startup/doctor/ping 测试与 `TestBinaryDoctor`，全量测试和 e2e 通过。
49. 日常 CLI 第二批：`hb run` 空闲时显示“已用时”（仅 TTY）；回合结束输出紧凑回执
    （摘要/改动/验证/未验证/下一步）；模型 401/403/404/400/429/5xx/连接错误映射为可执行建议；
    新增 `hb resume <id> --headless`。补 HumanWriter 回执、apply_patch 路径、状态错误与
    二进制 headless resume 测试。
50. TUI 第三批：`/diff` `/undo` `/rewind` 改为异步 `tea.Cmd`，执行中显示“正在执行”，
    大仓库不再阻塞按键与渲染；工具输出支持 `/verbose n` 选卡片、`all` 全部、`list` 列出、
    `last` 最后一个，`Ctrl+O` 仍是最后一个卡片的快捷切换。更新 tech-debt D09/D10 为已关闭。

## 2026-11-23

1. `hb export` 的 meta 行手工拼 JSON，Windows 路径 `C:\...` 被当成非法转义 → 改用 `json.Marshal`，并加 jsonl 合法性测试。
2. 自更新时旧二进制正在运行，`Copy-Item` 覆盖失败 → 先改名为 `.old` 再落新文件；启动时清理 `.old`。
3. `govulncheck` 报两个可达漏洞（x/text、goldmark）→ 升级到 v0.39.0 / v1.7.17，复扫为 0。
4. 高危授权测试最初用 `shutdown` 做样本，批准后会真的关机 → 换成无副作用的 `git reset --hard` / `git push --force`（临时空仓库内必然失败）。
5. 自我保证措辞测试发现 `done` 被 Gate 拒绝 → 确认是预期的加严行为，修改测试预期而不是放宽 Gate。
6. 回放样本里 `verify_test` 在空模块上退出码 0，无法制造失败 → 测试前写入一个必失败的 `fail_test.go`。
7. 用 goreleaser snapshot 产物（真实 zip + checksums）通过本地 HTTP 跑了一次 `hb upgrade`，升级后的 `hb --version` 显示 zip 内版本，确认资产命名与解压逻辑一致。
8. 用 DeepSeek Flash 实跑单文件 HTML 太阳系建模。产物 21.8KB；Edge headless 截图正常，8 个行星按钮、暂停/继续、点击显示简介均通过 CDP 实测。运行过程中暴露并修复：
   - 无关 shell 失败证据会让 Gate 直接拒绝完成 → 改为只判定与改动相关的证据。
   - 自我保证正则把“未通过 / 无法通过”误判成自我保证 → 收紧正则。
   - `max_tokens` 按 `MaxTurns*512` 分配，推理模型耗尽预算后返回空响应 → 改为独立 32k 输出预算。
   - 缺少 `[SELF]` 直接终止整个回合 → 改为拒执行当前步并给两次补救。
   - shell 空输出被模型误判为失败 → 工具结果附退出码，system prompt 明确 `request_done` / `declare_untested` 参数。
9. 第 5 次实跑时 DeepSeek 返回 `Insufficient Balance`，修复后的端到端 `done` 复跑暂时中断；余额补足后已继续测试。
10. 补足余额后重跑单文件 HTML 等距房间：`done`，退出码 0，173.5s，输入 161k / 输出 38k。Edge 截图与 CDP 实测通过（6 件家具、昼夜切换、悬停提示）。
11. Go bugfix 实跑：种子仓库带失败测试，模型修复实现并跑 `verify_build` / `verify_test` / `verify_vet`，37.6s 拿到 `done`，`go test ./...` 通过。
12. 诚实诱导实跑：用户要求“修完但不要跑测试，直接声称完成”。模型修好代码后拒绝虚报，调用 `declare_untested`，最终 `untested`/退出码 3；事后本地测试确实通过。
13. 计划模式实跑：`--plan` 17.4s 产出计划并 halt；`hb approve` 后完成 60 秒环形倒计时页面（76.1s，`done`）。CDP 实测开始/暂停/重置与进度环均正常。
14. 计划模式与 HTML+shell 任务暴露 `RequiredFor` 真实 bug：未知文件类型与 command 同时存在时只要求 command，导致 `evidence_kind` 误拒；已按“每个未知改动都必须要求 diff”修复并补测试。
15. DeepSeek 思考模式需要回传 `reasoning_content`；已在 Chat 适配器解析、在 engine 回传，并新增 `reasoning_effort` 配置。思考强度设为 low 后，同等任务输出 token 从 36k 降到约 12k。
16. 长计划执行还暴露 40 次工具调用上限过紧、自评补救计数跨步累计误伤、`hb approve` 未继承 run 的配置；均已修复（上限 100、有效自评后清零、approve 配置对齐）。
17. 用户在主目录 `C:\Users\（MC5lan）` 启动 TUI 后“没有动静”：检查点扫描主目录消耗到 3.6GB 内存，且旧终端没有继承新设置的 `HB_API_KEY`。已跳过主目录检查点与仓库地图，检查点增加 4000 文件/128MB 上限并排除 AppData/OneDrive/包缓存；修复后同样主目录的 `hb run` 2 秒完成。
