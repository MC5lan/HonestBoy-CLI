# 严格质量契约（rigor）

来源：[`BasicProtein/Prompt-Engineering-for-Developers`](https://github.com/BasicProtein/Prompt-Engineering-for-Developers)（MIT）。
我们只吸收与 HonestBoy「证据驱动完成」兼容的部分；不把提示词当成 Gate 的替代品。

## 使用

质量契约**常开**，不需要配置。旧配置里的 `rigor: normal` 会被收敛为 `strict`，且不能通过命令关闭：

```powershell
hb config get rigor           # strict
hb config set rigor strict    # 兼容写法，仍是 strict
hb config set rigor normal    # 报错：质量契约已常开，不能关闭
```

质量契约只影响三处：

1. system prompt 追加 `RigorPromptFragment`（`internal/core/rigor.go`）。
2. 写文件后扫描新增内容里的 `TODO` / `FIXME` / `not implemented` 等占位符；
   命中时发 `notice{warn}`，并把警告作为工具结果回传给模型。
3. 不改变 Gate、证据类型、终态机，也不自动放宽/收紧审批。
   需要「每步确认」时，另配 `approval.write=ask` / `approval.exec=ask`。

## 吸收映射

| 来源条款 | 落地方式 | 是否硬约束 |
|---|---|---|
| 完整实现、禁止占位符 | system prompt + 写入后占位符告警 | 否；Gate 仍只认证据 |
| 无法完成必须说明 | 复用 `declare_untested` / not_done | 是（原有机制） |
| 错误处理、边界、安全 | 质量契约提示 + 测试计划 | 否；无法自动判定 |
| 最小 diff、禁止无关文件 | 质量契约提示 + 现有读后写/补丁工具 | 否 |
| 任务复杂度分级 | 质量契约要求先判断；复杂任务先计划 | 否 |
| 先计划后执行 | 现有 `--plan` + `hb approve` | 是（计划模式） |
| 需求不明确先问 | 现有歧义暂停（A1–A5） | 是 |
| 每步人工确认 | 现有 approval（write/exec/net=ask） | 可选 |
| 外部资料注明来源 | 质量契约提示 | 否 |
| 测试/编译才算完成 | 现有 build/test/vet 证据链 | 是（原有机制） |
| 禁止创建测试文件 | **不采用** | 与证据链冲突；只约束“无关测试” |
| 禁止运行/编译/测试 | **不采用** | 没有运行就没有证据 |
| 只用 MCP 询问/记忆 | **暂不采用** | 当前无 MCP；`HB.md`/项目上下文可替代一部分 |
| 只输出代码、不写总结 | **不采用** | TUI/审计需要可读说明 |

## 质量契约（可复制到项目 `HB.md`）

```markdown
- 完整实现用户要求的全部范围；核心逻辑不得用 TODO、FIXME、占位符或 mock 代替。
- 无法完成的子项必须调用 declare_untested，并在 not_done 中逐项列出。
- 修改代码时补齐错误处理、边界情况与安全考量；保持最小 diff。
- 构建/测试/静态检查必须真实运行；测试是证据，不是可选项。
- 需求不明确或存在多种会改变结果的方案时，暂停并说明缺口。
- 引用外部文档或 API 时注明来源，不编造版本与接口。
- 最终说明保持简洁；不创建与任务无关的文件或文档。
```

质量契约是**常开的质量层**，不是新的完成判据。`done` 仍然只能由 Gate 根据可接地证据写入。
