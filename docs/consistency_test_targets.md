# 文档一致性检查目标（TC07）

已由 `docs/consistency_test.go` 机器化执行：

1. `hb --help` 命令集合 == `docs/commands.md`（双向）；README 中出现的命令必须是真实命令
2. 退出码表 == `internal/cli/exit.go`
3. TUI 快捷键表 == `internal/tui/help.go` 的 `KeyBindings()`
4. 错误码表 == `internal/errs`

运行：`go test ./docs -v`
