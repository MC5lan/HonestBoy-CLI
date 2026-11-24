package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// slashCmd 是斜杠命令及其中文说明（用于命令面板）。
type slashCmd struct {
	name     string
	desc     string
	needsArg bool
}

var slashCommands = []slashCmd{
	{"/model", "切换模型（例：/model deepseek-chat）", true},
	{"/config", "查看当前配置", false},
	{"/path", "显示完整工作区路径", false},
	{"/clear", "清空显示，不重置上下文", false},
	{"/new", "新会话：清空上下文并创建新会话", false},
	{"/reset", "同 /new：重置上下文并新开会话", false},
	{"/sessions", "选择并恢复历史会话", false},
	{"/verify", "校验证据链", false},
	{"/stats", "诚实度统计", false},
	{"/diff", "查看工作区 diff", false},
	{"/undo", "撤销最近一次自动提交", false},
	{"/rewind", "列出检查点 / rewind restore <id>", true},
	{"/approve", "批准计划：/approve <session-id>", true},
	{"/context", "上下文占用（估算）", false},
	{"/compact", "压缩下一回合历史", false},
	{"/theme", "切换主题：dark/light/none/auto", true},
	{"/reload", "重载模型/协议/端点/主题", false},
	{"/verbose", "展开/折叠工具输出（Ctrl+O）", false},
	{"/setup", "配置向导说明", false},
	{"/help", "快捷键与命令", false},
	{"/quit", "退出", false},
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

func (m *Model) slashPanelVisible() bool {
	return !m.showHelp && !m.showSessions && len(m.slashSuggestions()) > 0
}

const paletteWindow = 6

func (m *Model) movePalette(delta int) {
	sug := m.slashSuggestions()
	if len(sug) == 0 {
		return
	}
	m.paletteIdx += delta
	if m.paletteIdx < 0 {
		m.paletteIdx = 0
	}
	if m.paletteIdx >= len(sug) {
		m.paletteIdx = len(sug) - 1
	}
	if m.paletteIdx < m.paletteOffset {
		m.paletteOffset = m.paletteIdx
	}
	if m.paletteIdx >= m.paletteOffset+paletteWindow {
		m.paletteOffset = m.paletteIdx - paletteWindow + 1
	}
}

func (m *Model) completePalette() {
	sug := m.slashSuggestions()
	if len(sug) == 0 {
		return
	}
	if m.paletteIdx >= len(sug) {
		m.paletteIdx = len(sug) - 1
	}
	m.input.SetValue(sug[m.paletteIdx].name + " ")
	m.input.CursorEnd()
	m.paletteIdx, m.paletteOffset = 0, 0
	m.resizeComposer()
}

func (m *Model) submitPalette() tea.Cmd {
	sug := m.slashSuggestions()
	if len(sug) == 0 {
		return m.submit()
	}
	if m.paletteIdx >= len(sug) {
		m.paletteIdx = len(sug) - 1
	}
	c := sug[m.paletteIdx]
	if c.needsArg {
		m.input.SetValue(c.name + " ")
		m.input.CursorEnd()
		m.paletteIdx, m.paletteOffset = 0, 0
		m.hint = c.name + " 需要参数"
		m.resizeComposer()
		return nil
	}
	m.input.SetValue("")
	return m.runSlash(c.name, c.name, nil)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
