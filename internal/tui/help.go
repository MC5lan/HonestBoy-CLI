package tui

import (
	"fmt"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
)

// keyBinding 是快捷键表的单条记录；help 页与文档一致性测试共用同一来源。
type keyBinding struct {
	Key  string
	Desc string
}

var keyBindings = []keyBinding{
	{"Enter", "提交任务或执行命令；命令面板中执行高亮项"},
	{"Shift+Enter / Alt+Enter", "输入换行（多行任务）"},
	{"Ctrl+C", "中断当前回合；空闲时连按两次退出；审批/帮助页同样生效"},
	{"Ctrl+L", "清空对话显示"},
	{"Ctrl+O", "展开/折叠工具输出"},
	{"Ctrl+R", "只读重放当前会话"},
	{"Ctrl+Home / Ctrl+End", "对话区到顶/到底"},
	{"Ctrl+U / Ctrl+D", "对话区上/下半页"},
	{"PgUp / PgDn", "对话区翻页（滚轮同效）"},
	{"Esc", "忙碌时中断并清空队列；空闲时关闭面板/提示"},
	{"↑ / ↓", "翻历史输入 / 命令面板与会话列表选择"},
	{"Tab", "补全斜杠命令"},
	{"?", "打开或关闭本帮助"},
	{"/", "打开命令面板"},
}

// KeyBindings 返回快捷键键名列表，供文档一致性测试使用。
func KeyBindings() []string {
	out := make([]string, 0, len(keyBindings))
	for _, b := range keyBindings {
		out = append(out, b.Key)
	}
	return out
}

// helpPage 渲染快捷键与常用命令帮助；height 控制滚动窗口。
func (m *Model) helpPage(height int) string {
	width := maxInt(20, m.width-8)
	var lines []string
	lines = append(lines, m.th.AccentStyle().Render("快捷键（PgUp/PgDn 滚动）"))
	for _, b := range keyBindings {
		lines = append(lines, render.Truncate(fmt.Sprintf("  %-16s %s", b.Key, b.Desc), width))
	}
	lines = append(lines, "")
	lines = append(lines, m.th.AccentStyle().Render("常用命令"))
	for _, c := range slashCommands {
		lines = append(lines, render.Truncate(fmt.Sprintf("  %-12s %s", c.name, c.desc), width))
	}
	lines = append(lines, "")
	lines = append(lines, m.th.DimStyle().Render("PgUp/PgDn 滚动 · ? / Esc 关闭 · 输入 / 看命令面板"))
	if height <= 0 {
		height = len(lines)
	}
	if m.helpOffset > len(lines)-height {
		m.helpOffset = len(lines) - height
	}
	if m.helpOffset < 0 {
		m.helpOffset = 0
	}
	end := m.helpOffset + height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[m.helpOffset:end], "\n")
}
