# 贡献指南

## 开发环境
- Go 1.27+，`CGO_ENABLED=0` 可编译；Windows 11 / Linux x64。
- 可选：`golangci-lint`、`goreleaser`、`ripgrep`。

## 跑测试
```bash
make test          # 全量单测
make bench         # 性能基线
make verify-deps   # 依赖方向守门
make release-check # 发布前检查
```

## 代码风格
- 每个导出类型/函数必须有注释；单函数 ≤60 行、单文件 ≤800 行。
- 依赖方向：`core → 标准库`；`llm/tools/store → core + 标准库`；`engine → core/llm/tools/store`；
  `tui → core/render/theme`；`cmd → cli/tui/engine`。违规由 `scripts/checkdeps.ps1` 拦截。
- 表驱动测试，name 描述行为；输出形状用 golden / 快照。

## 提交规范
`<type>(<scope>): <subject>`，type ∈ `feat|fix|refactor|test|docs|chore|perf|build`。

## 新增工具 checklist
- [ ] `Name()` 全局唯一，`Schema()` 为合法 JSON Schema
- [ ] `Risk()` 正确标注；Provenance 声明（检索类为 inferred）
- [ ] 所有文件访问经 `Sandbox`，无裸 `os.Open`
- [ ] 表驱动单测 + 错误信息可操作
- [ ] 注册到 `NewDefaultRegistry` 并在 `docs/commands.md` 更新工具表
