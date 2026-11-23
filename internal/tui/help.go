package tui

import (
	"fmt"
	"strings"
)

// keyBinding 是快捷键表的单条记录；help 页与文档一致性测试共用同一来源。
type keyBinding struct {
	Key  string
	Desc string
}

var keyBindings = []keyBinding{
	{"Enter", "提交任务或执行命令"},
	{"Ctrl+C", "中断当前回合（空闲时退出）"},
	{"Ctrl+L", "清空对话显示"},
	{"Esc", "忙碌时中断；空闲时关闭提示"},
	{"↑ / ↓", "翻历史输入 / 在命令面板内选择"},
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

// helpPage 渲染快捷键与常用命令帮助。
func (m *Model) helpPage() string {
	var lines []string
	lines = append(lines, m.th.AccentStyle().Render("快捷键"))
	for _, b := range keyBindings {
		lines = append(lines, fmt.Sprintf("  %-8s %s", b.Key, b.Desc))
	}
	lines = append(lines, "")
	lines = append(lines, m.th.AccentStyle().Render("常用命令"))
	for _, c := range slashCommands {
		lines = append(lines, fmt.Sprintf("  %-12s %s", c.name, c.desc))
	}
	lines = append(lines, "")
	lines = append(lines, m.th.DimStyle().Render("? / Esc 关闭 · 输入 / 看命令面板"))
	return strings.Join(lines, "\n")
}
