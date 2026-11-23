# 依赖清单（TD03）

直接依赖 6 个（上限 12），全部纯 Go、`CGO_ENABLED=0` 可编译。

| 依赖 | 用途 | 选择理由 |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | TUI 主循环 | 终端事件模型成熟；零 CGO |
| `github.com/charmbracelet/bubbles` | 输入框/视口/spinner | 与 bubbletea 同生态，避免自造组件 |
| `github.com/charmbracelet/lipgloss` | 样式与布局 | 统一主题 token，兼容 256 色 |
| `github.com/charmbracelet/glamour` | Markdown 渲染 | T701 要求；渲染失败降级原文 |
| `github.com/alecthomas/chroma/v2` | 代码高亮 | T702 要求；lexer/style 丰富 |
| `github.com/rivo/uniseg` | 宽字符/ZWJ 宽度 | grapheme 集群切分，避免 emoji 宽度错位 |

`gofrs/flock` 已纳入依赖（会话文件锁）；`mattn/go-isatty` / `go-runewidth` 由生态间接引入。
无 Node/npm、无 CGO、无 UI 框架。`govulncheck` 应在每月安全更新时运行；当前环境未安装该工具，
列为发布检查项。
