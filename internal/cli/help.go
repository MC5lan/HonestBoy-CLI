package cli

import (
	"fmt"
	"io"
	"strings"
)

// PrintHelpTopic 输出单个主题；未知主题返回 false。
func PrintHelpTopic(w io.Writer, topic string) bool {
	topics := map[string]string{
		"run": `hb run 用法
  hb run [--diff] [--profile] [--plan] [--auto-commit] [--auto-resume] <prompt>
  --diff          注入当前 git diff
  --profile       输出上下文/流式/首 token/工具耗时
  --plan          只读计划模式，结束后 hb approve <session-id>
  --yes           自动批准普通写/执行（高危永不放行）
  --auto-resume   没有新任务时续跑最近一条未完成会话`,
		"config": `hb config 用法
  hb config list
  hb config get <key>
  hb config set <key> <value>
  常用键：model / protocol / base_url / workdir / theme /
  context.window / reasoning_effort / rigor（常开）/ approval.exec /
  tools.custom_dir / web.enabled / web.allowed_domains`,
		"evidence": `证据与 Gate
  done 必须由本次真实运行、通过、可接地的 build/test/vet 证据支撑。
  hb verify   校验 Receipt 哈希链
  hb stats    终态分布与 FCR
  hb probe    三态探针（done/denied/untested）
  详见 docs/evidence-format.md 与 docs/architecture.md`,
		"tools": `工具与审批
  内置：read_file/write_file/edit_file/apply_patch/list_dir/search/shell/verify_*
  自定义：hb config set tools.custom_dir tools.d，见 docs/custom-tools.md
  web_fetch：默认关闭，需 web.enabled + 域名白名单
  高风险命令永不自动批准；不可逆动作每次单独授权。`,
		"exit-codes": `退出码
  0 done · 1 用法/已暂停/失败 · 3 untested · 4 denied
  2 环境错误 · 5 证据链断裂 · 6 环境未就绪 · 130 中断
  hb explain <code> 查看错误码说明`,
	}
	text, ok := topics[strings.ToLower(topic)]
	if !ok {
		return false
	}
	fmt.Fprintln(w, text)
	return true
}

func PrintHelp(w io.Writer) {
	fmt.Fprint(w, `hb - Honest Boy CLI

用法: hb [global flags] <command> [args...]

命令:
  hb init               首次设置向导（工作区 / 协议 / 端点 / 密钥）
  hb run <prompt>       单次执行（非交互）
  hb chat               显式交互
  hb sessions           列出/查看/删除会话
  hb resume <id>        恢复会话（TUI）
  hb resume --last      续跑当前工作区最近一条未完成会话（最多 2 次）
  hb verify             校验证据链
  hb stats              诚实度统计
  hb probe              三态探针
  hb config             配置查看/设置
  hb doctor [--fix]     环境自检；--fix 重建损坏配置
  hb undo                撤销最近一次自动提交（保留文件改动）
  hb diff                查看当前工作区 diff
  hb approve <id>        批准计划并执行（先 hb run --plan）
  hb rewind list         列出检查点（默认最近会话）
  hb rewind restore <id> 恢复代码到检查点
  hb gc [--yes]          清理过期会话(>90天)与日志(>14天)，默认预演
  hb export <id> [--redact] 导出会话 jsonl（可脱敏）
  hb upgrade [--check] [--force] [--timeout <dur>]  下载并校验最新版本
  hb explain <code>     查询错误码（如 HB-NET-001）
  hb help [topic]       帮助；topic: run/config/evidence/tools/exit-codes

全局 flag:
  --model <name>       默认模型
  --protocol <name>    openai-chat（默认）| anthropic（Messages）| responses（Responses API）
  --base-url <url>     兼容端点；含 anthropic 时自动选 anthropic 协议
  --cwd <dir>          工作区根目录（沙箱边界）
  --json, --quiet, --yes, --plan, --diff, --profile, --auto-commit, --no-color, --debug
  --timeout <dur>, --max-turns <n>
  --version, --help
`)
}
