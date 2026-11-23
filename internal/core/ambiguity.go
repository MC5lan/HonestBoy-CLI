package core

import (
	"path/filepath"
	"regexp"
)

// AmbiguityLevel 是歧义检测的三级结果（文档 §4.2）。
type AmbiguityLevel uint8

const (
	AmbPass AmbiguityLevel = iota // 直接执行
	AmbWarn                       // 执行但声明假设
	AmbHalt                       // 停下提问（不计失败，不扣预算）
)

func (a AmbiguityLevel) String() string {
	switch a {
	case AmbWarn:
		return "WARN"
	case AmbHalt:
		return "HALT"
	default:
		return "PASS"
	}
}

// 悬空指代模式（A1）。
var danglingRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(那个|刚才那个|这个|上面那个)\s*(文件|函数|方法|目录|东西)`),
	regexp.MustCompile(`(它|他|她)\s*(的|那个)`),
}

// Intent 是待判定意图（文档 §4.2 的检测输入）。
type Intent struct {
	TargetPattern  string
	HasDanglingRef bool
	Authority      Authority
	Granted        bool
	Action         string
}

// DetectAmbiguity 按 A1-A5 可判定谓词检测歧义（零依赖，纯函数）。
func DetectAmbiguity(in Intent, files []string, sa SelfAssessment) (AmbiguityLevel, string) {
	// A5：自评置信度 unknown → Halt
	if sa.Conf == "unknown" {
		return AmbHalt, "自评置信度为 unknown"
	}
	// A4：不可逆操作未授权 → Halt
	if in.Authority == AuthIrreversible && !in.Granted {
		return AmbHalt, "不可逆操作未获授权: " + in.Action
	}
	// A2：目标路径不存在或多处匹配 → Halt
	if in.TargetPattern != "" {
		cands := matchPaths(files, in.TargetPattern)
		switch {
		case len(cands) == 0:
			return AmbHalt, "目标不存在: " + in.TargetPattern
		case len(cands) > 1:
			return AmbHalt, "目标存在 " + itoaLocal(len(cands)) + " 处匹配，需指定其一"
		}
	}
	// A1：悬空指代 → Warn
	if in.HasDanglingRef {
		return AmbWarn, "存在未消解的指代，需声明假设"
	}
	return AmbPass, ""
}

// matchPaths 用 Glob 匹配路径（零依赖，用标准库 filepath.Match）。
func matchPaths(files []string, pattern string) []string {
	var out []string
	for _, f := range files {
		if ok, _ := filepath.Match(pattern, f); ok {
			out = append(out, f)
		}
		// 也支持 basename 精确匹配
		if ok, _ := filepath.Match(pattern, filepath.Base(f)); ok {
			out = append(out, f)
		}
	}
	return dedupe(out)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// HasDanglingReference 检测文本里的悬空指代（A1 的辅助）。
func HasDanglingReference(text string) bool {
	for _, re := range danglingRefPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

func itoaLocal(n int) string {
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
