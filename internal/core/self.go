package core

import "strings"

// SelfAssessment 是模型在每个动作前输出的自评块（文档 §4.1）。
// 关键：它必须影响控制流，不是装饰。
type SelfAssessment struct {
	Conf       string // high | mid | low | unknown
	Missing    string
	Assumption string
	Falsify    string
}

// ParseSelf 解析 [SELF] 块。解析失败返回 ok=false，调用方应视为 unknown。
func ParseSelf(s string) (SelfAssessment, bool) {
	var sa SelfAssessment
	i := strings.Index(s, "[SELF]")
	if i < 0 {
		return sa, false
	}
	body := s[i+len("[SELF]"):]
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "conf:"):
			sa.Conf = strings.TrimSpace(strings.TrimPrefix(line, "conf:"))
		case strings.HasPrefix(line, "missing:"):
			sa.Missing = strings.TrimSpace(strings.TrimPrefix(line, "missing:"))
		case strings.HasPrefix(line, "assumption:"):
			sa.Assumption = strings.TrimSpace(strings.TrimPrefix(line, "assumption:"))
		case strings.HasPrefix(line, "falsify:"):
			sa.Falsify = strings.TrimSpace(strings.TrimPrefix(line, "falsify:"))
		}
	}
	ok := sa.Conf == "high" || sa.Conf == "mid" || sa.Conf == "low" || sa.Conf == "unknown"
	return sa, ok
}

// BlocksWrite 判断自评是否应该阻断写操作（conf=unknown 或 assumption 无 falsify）。
func (sa SelfAssessment) BlocksWrite() bool {
	if sa.Conf == "unknown" || sa.Conf == "" {
		return true
	}
	if sa.Assumption != "" && sa.Assumption != "none" && (sa.Falsify == "" || sa.Falsify == "none") {
		return true
	}
	return false
}

// SelfPromptFragment 是追加到 system prompt 末尾的自评指令（文档 §4.1.1）。
// 注意：不含"你必须诚实"类表述（违反 F1）。
const SelfPromptFragment = `
在每个动作之前，先输出自评块，格式严格如下：
[SELF]
conf: high | mid | low | unknown
missing: <缺少哪些信息才能确定，无则填 none>
assumption: <本步骤依赖的未证实前提，无则填 none>
falsify: <如果 assumption 错了，会以什么形式暴露，无则填 none>

规则：
- conf=unknown 时不得继续执行写操作，应先提问
- assumption 非 none 时，必须同时给出 falsify
- 自评块不计入最终答案，仅用于决策`
