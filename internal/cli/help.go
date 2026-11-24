package cli

import (
	"fmt"
	"io"
)

func PrintHelp(w io.Writer) {
	fmt.Fprint(w, `hb - Honest Boy CLI

用法: hb [global flags] <command> [args...]

命令:
  hb init               首次设置向导（工作区 / 协议 / 端点 / 密钥）
  hb run <prompt>       单次执行（非交互）
  hb chat               显式交互
  hb sessions           列出/查看/删除会话
  hb resume <id>        恢复会话
  hb verify             校验证据链
  hb stats              诚实度统计
  hb probe              三态探针
  hb config             配置查看/设置
  hb doctor             环境自检
  hb undo                撤销最近一次自动提交（保留文件改动）
  hb diff                查看当前工作区 diff
  hb approve <id>        批准计划并执行（先 hb run --plan）
  hb rewind list         列出检查点（默认最近会话）
  hb rewind restore <id> 恢复代码到检查点
  hb gc [--yes]          清理过期会话(>90天)与日志(>14天)，默认预演
  hb export <id> [--redact] 导出会话 jsonl（可脱敏）
  hb upgrade [--check] [--force] [--timeout <dur>]  下载并校验最新版本
  hb explain <code>     查询错误码（如 HB-NET-001）
  hb help               帮助

全局 flag:
  --model <name>       默认模型
  --protocol <name>    openai-chat（默认）| anthropic（Messages）| responses（Responses API）
  --base-url <url>     兼容端点；含 anthropic 时自动选 anthropic 协议
  --cwd <dir>          工作区根目录（沙箱边界）
  --json, --quiet, --yes, --plan, --diff, --auto-commit, --no-color, --debug
  --timeout <dur>, --max-turns <n>
  --version, --help
`)
}
