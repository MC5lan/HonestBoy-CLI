# 依赖清单（TD03）

直接依赖 7 个（上限 12），全部纯 Go、`CGO_ENABLED=0` 可编译。

| 依赖 | 用途 | 选择理由 |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | TUI 主循环 | 终端事件模型成熟；零 CGO |
| `github.com/charmbracelet/bubbles` | 输入框/视口/spinner | 与 bubbletea 同生态，避免自造组件 |
| `github.com/charmbracelet/lipgloss` | 样式与布局 | 统一主题 token，兼容 256 色 |
| `github.com/charmbracelet/glamour` | Markdown 渲染 | T701 要求；渲染失败降级原文 |
| `github.com/alecthomas/chroma/v2` | 代码高亮 | T702 要求；lexer/style 丰富 |
| `github.com/rivo/uniseg` | 宽字符/ZWJ 宽度 | grapheme 集群切分，避免 emoji 宽度错位 |
| `github.com/gofrs/flock` | 会话文件锁 | 单写多读，避免并发写坏 jsonl |

`mattn/go-isatty` / `go-runewidth` 由生态间接引入。
无 Node/npm、无 CGO、无 UI 框架。

## 漏洞扫描

`govulncheck ./...` 已纳入发布检查（脚本：`scripts/govuln.ps1`）。最近一次结果：**0 个可达漏洞**。
升级记录：`golang.org/x/text` v0.30.0 → v0.39.0（GO-2026-5970）、
`github.com/yuin/goldmark` v1.7.13 → v1.7.17（GO-2026-5320）。
