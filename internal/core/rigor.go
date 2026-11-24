package core

import "regexp"

// RigorPromptFragment 是 strict 模式追加到 system prompt 的质量契约。
// 只吸收提示词工程里与现有架构兼容的部分：完整性、边界、最小改动、验证优先。
// 明确不采用“禁止测试/禁止运行/每步强制确认”，否则会破坏证据链与 CLI 自动化。
const RigorPromptFragment = `
质量契约（strict 模式，属于系统规则而非建议）：
1. 完整实现用户要求的全部范围。核心逻辑不得用 TODO、FIXME、占位符、mock 数据或空实现代替。
2. 如果无法完成某一部分，必须调用 declare_untested，并在 not_done 中逐项列出；不得静默缩小范围或假装完成。
3. 修改代码时补齐错误处理、边界情况与安全考量；保持最小 diff，不创建与任务无关的文件或文档。
4. 先判断任务复杂度：简单任务直接执行；涉及多文件或不确定性的任务先给出步骤与测试计划。
5. 构建、测试、静态检查是完成证据，必须真实运行；测试文件不是可选项，但也不要创建无关测试。
6. 需求不明确、存在多种会改变结果的方案，或需要猜测时，暂停并说明缺口，不自行假设。
7. 引用外部文档或 API 时注明来源；不要编造版本、接口或不存在的依赖。
8. 最终说明保持简洁，不复述整份代码，不写任务要求之外的总结。`

// placeholderPatterns 是 strict 模式下写入文件后的质量提醒。
// 只警告并回传给模型，不直接阻断 Gate；最终是否完成仍由证据判定。
var placeholderPatterns = []struct {
	label string
	re    *regexp.Regexp
}{
	{"TODO", regexp.MustCompile(`(?i)\bTODO\b`)},
	{"FIXME", regexp.MustCompile(`(?i)\bFIXME\b`)},
	{"XXX", regexp.MustCompile(`(?i)\bXXX\b`)},
	{"PLACEHOLDER", regexp.MustCompile(`(?i)\bPLACEHOLDER\b`)},
	{"not implemented", regexp.MustCompile(`(?i)\bnot\s+implemented\b`)},
	{"panic(not implemented)", regexp.MustCompile(`(?i)panic\s*\(\s*["']not implemented`)},
	{"throw(not implemented)", regexp.MustCompile(`(?i)throw\s+new\s+Error\s*\(\s*["']not implemented`)},
}

// PlaceholderWarnings 返回文本中命中的占位符类别（去重）。
func PlaceholderWarnings(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range placeholderPatterns {
		if p.re.MatchString(text) && !seen[p.label] {
			seen[p.label] = true
			out = append(out, p.label)
		}
	}
	return out
}

// RigorEnabled 判断质量契约是否启用。
// 质量契约常开：无论配置是 strict、normal、空还是旧值，都返回 true。
// 保留 level 参数只为兼容旧配置与调用方；normal 不再表示关闭。
func RigorEnabled(level string) bool {
	_ = level // 常开：忽略配置值，normal 不再表示关闭
	return true
}
