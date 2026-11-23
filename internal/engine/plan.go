package engine

import (
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

// planModeRules 追加到系统提示，明确禁止计划阶段的写操作。
const planModeRules = `
当前处于【计划模式】：
- 只使用只读工具（read_file / list_dir / search / 只读 shell）探索代码。
- 不要调用任何写工具（edit_file / write_file / apply_patch / shell 写命令）。
- 输出一个清晰的执行计划：要改的文件、每一步做什么、如何验证。
- 计划输出后回合会暂停，等待用户批准；批准前不会修改任何文件。`

// WithPlanRules 在系统提示末尾追加计划模式规则。
func WithPlanRules(system string) string {
	return strings.TrimRight(system, "\n") + "\n" + planModeRules
}

// planToolSpecs 在计划模式下只暴露只读工具（对标 Claude Code plan mode）。
func planToolSpecs(registry *tools.Registry) []llm.ToolSpec {
	list := registry.List()
	out := make([]llm.ToolSpec, 0, len(list))
	for _, t := range list {
		switch t.Risk() {
		case tools.RiskRead:
			// 只读工具全部放行
		case tools.RiskExec:
			// 验证器与只读白名单允许；其他命令在计划模式下拦截
			if !strings.HasPrefix(t.Name(), "verify_") {
				continue
			}
		default:
			continue // RiskWrite / RiskNet 一律不暴露
		}
		out = append(out, llm.ToolSpec{Name: t.Name(), Description: t.Description(), Schema: t.Schema()})
	}
	return out
}

// PlanApproved 从 Receipt 中取出已批准的计划文本。
func PlanApproved(r core.Receipt) (string, bool) {
	if r.Terminal != core.TermHalt {
		return "", false
	}
	return r.Claim, r.Claim != ""
}
