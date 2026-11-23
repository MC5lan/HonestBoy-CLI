# Honest Boy CLI (`hb`)

![Honest Boy 吉祥物](assets/preview.png)

同一个模型，换一个 harness，它虚报完成的概率会变。
Honest Boy 的全部工程，就是为了让“没验证就说完成”这件事在结构上发不出去。

> 当前状态：核心诚实机制、三套模型协议、受沙箱约束的工具集、非交互 CLI、Bubble Tea TUI、
> 会话/证据链、检查点与发布脚手架均已实现；`go test ./...`、二进制级 e2e、依赖方向检查全绿。

## 已可用的能力

- `hb run "<任务>"`：单次执行；无证据申请完成时退出码 4，弃权退出码 3，无密钥退出码 6
- `hb verify`：逐条重算 Receipt 哈希链，篡改会定位到具体记录（退出码 5）
- `hb stats`：done / untested / denied / halt 分布
- `hb sessions`：会话列表、`show <id>`、`rm <id>`
- `hb probe --json`：三态探针（正常 / 诱导 / 施压），可进 CI 回归
- `hb doctor` / `hb config`：环境自检与配置管理
- `hb gc` / `hb export --redact` / `hb upgrade --check`：数据清理、脱敏导出、自更新
- 模型协议：`--protocol openai-chat|anthropic|responses`，或用 `--base-url` 自动推断
- `hb`（有 TTY）/ `hb chat`：Bubble Tea 交互界面（流式渲染、视口滚动、斜杠命令、快捷键）
- TUI 内 `/verify` `/stats` `/sessions` `/context` `/compact` 均可用，不用退出界面
- 诚实机制已接入主循环：歧义暂停、自评阻断、测试计划三态、不可逆单次授权、跨回合失败账本检索
- 长任务能力：自动上下文压缩 + 仓库符号地图 + 每轮 git 状态注入 + 检查点（`hb rewind`）+ `apply_patch` 补丁编辑
- 计划模式：`hb run --plan "任务"` 先只读探索并输出计划，`hb approve <id>` 批准后才执行
- git 工作流：`hb run --auto-commit "..."` 自动提交并产出 diff 审阅，`hb undo` 撤销、`hb diff` 查看

品牌资源在 `assets/`（吉祥物预览图与 Windows 图标，供文档与发布阶段使用）。

## 六段结构（TC01）

1. 一句话是什么：本地 AI 编程智能体 CLI，命令为 `hb`。
2. 为什么不一样：完成权外置给 Gate，只有 `observed` 证据能支撑 `done`。
3. 安装：阶段 11（TB01）提供三平台单文件分发。
4. 30 秒上手：`hb --version`、`hb run "..."`、`hb verify`。
5. 它不会做什么：不做遥测、云同步、绝对防作弊，详见 `docs/frozen-scope.md`。
6. 文档索引：见 `docs/` 目录；发布步骤见 `docs/release-checklist.md`。

## 构建

```bash
make build
make test
```