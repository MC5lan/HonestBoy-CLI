# Honest Boy CLI (`hb`)

![Honest Boy 吉祥物](assets/preview.png)

同一个模型，换一个 harness，它虚报完成的概率会变。
Honest Boy 的全部工程，就是为了让“没验证就说完成”这件事在结构上发不出去。

> 当前状态：**阶段 1–8 已实现**（core 状态机与 Gate、三套模型协议、受沙箱约束的工具集、
> 非交互 CLI、会话落盘与三态探针、Bubble Tea TUI 与富内容渲染、审批往返与会话恢复）。
> 手册其余阶段按 `HonestBoy-CLI-全量开发手册-v3.md` 继续推进。

## 已可用的能力

- `hb run "<任务>"`：单次执行；无证据申请完成时退出码 4，弃权退出码 3，无密钥退出码 6
- `hb verify`：逐条重算 Receipt 哈希链，篡改会定位到具体记录（退出码 5）
- `hb stats`：done / untested / denied / halt 分布
- `hb sessions`：会话列表、`show <id>`、`rm <id>`
- `hb probe --json`：三态探针（正常 / 诱导 / 施压），可进 CI 回归
- `hb doctor` / `hb config`：环境自检与配置管理
- 模型协议：`--protocol openai-chat|anthropic|responses`，或用 `--base-url` 自动推断
- `hb`（有 TTY）/ `hb chat`：Bubble Tea 交互界面（流式渲染、视口滚动、斜杠命令、快捷键）
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
6. 文档索引：见 `docs/` 目录。

## 构建

```bash
make build
make test
```