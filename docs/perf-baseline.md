# 性能基线（TA09）

在目标开发机（Windows x64, Go 1.27）上用 `go test -bench . -benchmem ./internal/core` 测得，
作为回归对比基线。退化 > 20% 需在提交说明中给出理由。

| 基准 | 含义 | 目标 |
|---|---|---|
| `BenchmarkEmitEvents` | 内存事件发出 | 亚微秒级/条 |
| `BenchmarkReceiptHash` | Receipt 规范化哈希 | 微秒级/次 |
| `BenchmarkGateEvaluate` | Gate 纯函数判定 | 微秒级/次 |
| `BenchmarkSHA256` | 原文哈希 | 亚微秒级/次 |

## 实测（2026-11-23，Windows x64，Go 1.27.4，12 线程）

| 基准 | 结果 | 分配 |
|---|---|---|
| `BenchmarkEmitEvents` | 184.4 ns/op | 508 B/op, 0 allocs/op |
| `BenchmarkReceiptHash` | 1389 ns/op | 528 B/op, 4 allocs/op |
| `BenchmarkGateEvaluate` | 398.8 ns/op | 176 B/op, 2 allocs/op |
| `BenchmarkSHA256` | 84.37 ns/op | 0 B/op, 0 allocs/op |

本机结果全部落在目标区间内。

端到端指标（启动到可输入 <300ms、1000 事件恢复 <500ms、10 万文件搜索 <3s）在 TA09
阶段用真实工作区测量；当前二进制 strip 后约 16.3MB，超过手册 12MB 目标（见 tech-debt D01）。
