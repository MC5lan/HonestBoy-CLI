package tools

import "strings"

// Compress 压缩工具输出进上下文前的体积，且不删除任何含
// error / FAIL / panic 的行。
func Compress(s string) string {
	// 进度条：只保留 \r 分隔的最后一段。
	if strings.Contains(s, "\r") {
		parts := strings.Split(s, "\r")
		s = parts[len(parts)-1]
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 40 {
		head := lines[:30]
		tail := lines[len(lines)-10:]
		lines = append(append([]string{}, head...), "… 省略中间 "+itoaLocal(len(lines)-40)+" 行 …")
		lines = append(lines, tail...)
	}

	var out []string
	blankRun := 0
	sameText := ""
	sameRun := 0
	flushRepeat := func() {
		if sameRun > 3 {
			out[len(out)-1] = sameText + "  [x" + itoaLocal(sameRun) + "]"
		} else if sameRun > 1 {
			for i := 1; i < sameRun; i++ {
				out = append(out, sameText)
			}
		}
		sameRun = 0
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushRepeat()
			blankRun++
			if blankRun == 1 {
				out = append(out, "")
			}
			continue
		}
		blankRun = 0
		if isCritical(line) {
			flushRepeat()
			out = append(out, line)
			sameText = ""
			continue
		}
		if line == sameText {
			sameRun++
		} else {
			flushRepeat()
			out = append(out, line)
			sameText = line
			sameRun = 1
		}
	}
	flushRepeat()
	return strings.Join(out, "\n")
}

func isCritical(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic")
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
