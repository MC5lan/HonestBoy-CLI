package core

import "strings"

// ReasonFor 把机器 Rule 映射为可读文案。
// forModel=true 给模型（带具体缺口），false 给用户（简短）。
func ReasonFor(d Decision, forModel bool) string {
	if d.Reason != "" && forModel {
		return d.Reason
	}
	switch d.Rule {
	case "coverage":
		if forModel {
			return "以下目标缺少验证：" + strings.Join(d.Missing, ", ") + "。请运行相应验证。"
		}
		return "有 " + itoa(len(d.Missing)) + " 个改动未被验证"
	case "evidence_failed":
		if forModel {
			return "验证未通过：" + d.Summary + "。请修复后重试。"
		}
		return "验证失败：" + d.Summary
	case "provenance":
		if forModel {
			return "该证据来源为 " + d.Provenance + "，不能作为完成依据。"
		}
		return "证据来源不足"
	case "evidence_kind":
		if forModel {
			return "需要 " + strings.Join(d.Needed, "/") + " 类型的证据，当前是 " + d.Got + "。"
		}
		return "证据类型不符"
	default:
		if forModel {
			return "证据尚未齐备"
		}
		return "未完成验证"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
