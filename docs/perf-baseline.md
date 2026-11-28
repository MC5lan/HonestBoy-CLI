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

## 10 万文件搜索实测（Windows，2026-11-28）

测试目录：`%TEMP%\hb-perf-100k`，100 个子目录 × 1000 个 `.txt`，共 **100,000 文件 / 55.1MB**；
NTFS，Windows Defender 实时保护开启。每项跑 3 次，取中位数。

| 场景 | 无 rg（Go 兜底） | 有 rg 16.0.0 |
|---|---|---|
| 无命中，必须扫完 10 万文件 | **25.65s** | **4.79s** |
| 命中在最后一个文件 | 25.63s | 4.72s |
| 常见词，拿满 200 条即停 | 36ms | 35ms |
| `list_dir` 根目录（100 个目录） | 1ms | 1ms |

### Linux CI 对照

同样 100,000 文件 / 55.1MB，GitHub Actions `ubuntu-latest`，Go 1.27，run `46125964343`：

| 场景 | 无 rg（Go 兜底） | 有 rg |
|---|---|---|
| 无命中，必须扫完 10 万文件 | **1.19s** | **224ms** |
| 命中在最后一个文件 | 1.18s | 218ms |
| 常见词，拿满 200 条即停 | 1ms | 4ms |
| `list_dir` 根目录（100 个目录） | <1ms | <1ms |

结论：**<3s 目标在 Linux 达成**（无 rg 1.19s、有 rg 0.22s），但在开启 Defender 的 Windows
**未达成**（无 rg 25.7s、有 rg 4.7s）。根因不是正则或 CPU：

- `filepath.WalkDir` 枚举 10 万文件仅 113ms；
- 逐文件打开+读取占 25.1s（约 0.25ms/文件）；
- Go 并行读取（12/32/64/128 线程）最低约 14s，瓶颈在 Defender 逐文件扫描/文件系统；
- `findstr /S` 实测 38.4s，比 Go 兜底更慢；
- rg `-j 12` / `--no-mmap` 最好约 4.6s，仍受同一瓶颈限制。

已修复两个会让结果更慢的缺陷（`b7a832e`）：

1. rg 正常返回“0 条命中”时被误判为 rg 不可用，又跑一遍 Go 兜底，实测 31.3s；现在只扫一遍。
2. rg 路径原来等全量输出再解析；改为流式读取，拿满 `max_results` 立即停止，常见词从“等全仓扫完”降到 35ms。

v1.0 处置：目标需要按平台拆开，不能写成一个数字：

- Linux：<3s 已达标（无 rg 也达标）。
- Windows + Defender：有 rg 约 4.7s、无 rg 约 25.7s；建议目标改为“有 rg <5s”，无 rg 只保证可用并提示安装。
- `hb doctor` 已提示安装 ripgrep；是否增加索引/大仓库保护仍在 D15 跟踪。

在 Windows 子项决策前，10 万文件搜索门禁记为“Linux 达标 / Windows 未达标”，不写“已全部达标”。

端到端指标（启动到可输入 <300ms、1000 事件恢复 <500ms、10 万文件搜索 <3s）按平台测量；
二进制体积不作为发布门禁，也不设硬上限（产品决定，见 tech-debt D01）。
