# 性能基线（TA09）

在目标开发机（Windows x64, Go 1.27）上用 `go test -bench . -benchmem ./internal/core` 测得，
作为回归对比基线。退化 > 20% 需在提交说明中给出理由。

| 基准 | 含义 | 目标 |
|---|---|---|
| `BenchmarkEmitEvents` | 内存事件发出 | 亚微秒级/条 |
| `BenchmarkReceiptHash` | Receipt 规范化哈希 | 微秒级/次 |
| `BenchmarkGateEvaluate` | Gate 纯函数判定 | 微秒级/次 |
| `BenchmarkSHA256` | 原文哈希 | 亚微秒级/次 |

## 实测（2026-11-24，Windows x64，Go 1.27.4，12 线程）

| 基准 | 结果 | 分配 |
|---|---|---|
| `BenchmarkEmitEvents` | 110–236 ns/op（5 次） | 510–564 B/op, 0 allocs/op |
| `BenchmarkReceiptHash` | 1337 ns/op | 512 B/op, 4 allocs/op |
| `BenchmarkGateEvaluate` | 486–591 ns/op（5 次） | 240 B/op, 5 allocs/op |
| `BenchmarkSHA256` | 82.73 ns/op | 0 B/op, 0 allocs/op |

2026-11-24 重新基线说明：`Gate` 自 `81fa561` 后未改动；旧记录（398.8 ns / 2 allocs）
与当前构建不一致，原因未完全定位，以本次实测为准。发布前若 Gate 再改动，需要重新测量并说明。

### Markdown 渲染

`BenchmarkMarkdown200Code`（200 行 Go 代码块，100 列，dark）：**35.5ms/op**，
未达到手册 T702 的 <30ms 目标；LRU 缓存命中路径明显更快，后续需优化首次渲染或拆分代码块缓存。

### 启动与会话恢复

- `hb --version` 冷启动 5 次：57–64ms（目标 <300ms）。
- `BenchmarkLoadThousandEvents`：**9.8ms/op**（目标 <500ms）。
- 10 万文件搜索尚未在目标目录实测。

端到端指标（启动到可输入 <300ms、1000 事件恢复 <500ms、10 万文件搜索 <3s）在目标环境用真实工作区测量；
二进制体积不作为发布门禁，也不设硬上限（产品决定，见 tech-debt D01）。
