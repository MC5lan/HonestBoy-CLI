package tui

import "strings"

// slashCmd 是斜杠命令及其中文说明（用于命令面板）。
type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/model", "切换模型（例：/model deepseek-chat）"},
	{"/config", "查看当前配置"},
	{"/path", "显示完整工作区路径"},
	{"/clear", "清空当前会话显示"},
	{"/sessions", "会话列表"},
	{"/verify", "校验证据链"},
	{"/stats", "诚实度统计"},
	{"/context", "上下文占用"},
	{"/compact", "压缩上下文"},
	{"/setup", "配置向导说明"},
	{"/help", "快捷键与命令"},
	{"/quit", "退出"},
}

// SlashCommand 解析斜杠命令。
func SlashCommand(line string) (name string, args []string, ok bool) {
	if !strings.HasPrefix(line, "/") {
		return "", nil, false
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", nil, false
	}
	return parts[0], parts[1:], true
}

// slashSuggestions 返回与当前输入前缀匹配的命令（用于命令面板）。
func (m *Model) slashSuggestions() []slashCmd {
	v := strings.TrimSpace(m.input.Value())
	if !strings.HasPrefix(v, "/") || strings.Contains(v, " ") {
		return nil
	}
	var out []slashCmd
	for _, c := range slashCommands {
		if strings.HasPrefix(c.name, v) {
			out = append(out, c)
		}
	}
	return out
}

// suggestCommands 返回最相近的两个命令（供未知命令提示）。
func suggestCommands(input string) []string {
	best := make([]string, 0, 2)
	scores := make([]int, 0, 2)
	for _, c := range slashCommands {
		d := editDistance(input, c.name)
		if len(best) < 2 {
			best = append(best, c.name)
			scores = append(scores, d)
			continue
		}
		worst := 0
		for i := 1; i < len(scores); i++ {
			if scores[i] > scores[worst] {
				worst = i
			}
		}
		if d < scores[worst] {
			best[worst], scores[worst] = c.name, d
		}
	}
	if len(best) == 2 && scores[0] > scores[1] {
		best[0], best[1] = best[1], best[0]
	}
	return best
}

func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur := make([]int, len(br)+1)
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = minInt(cur[j-1]+1, minInt(prev[j]+1, prev[j-1]+cost))
		}
		prev = cur
	}
	return prev[len(br)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
