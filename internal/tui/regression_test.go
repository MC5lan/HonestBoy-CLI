package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 命令面板里回车必须执行高亮项，而不是输入框原文（之前会报“未知命令 /”）。
func TestPaletteEnterRunsHighlightedItem(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "deepseek-chat", Protocol: "openai-chat"})
	m.setSize(100, 34)
	m.input.SetValue("/")
	m.paletteIdx = 1 // /config
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if strings.Contains(m.hint, "未知命令") {
		t.Fatalf("enter ran typed text instead of selection: %q", m.hint)
	}
	if !strings.Contains(m.hint, "协议") {
		t.Fatalf("highlighted /config did not run: %q", m.hint)
	}
}

// 需要参数的命令回车应补全命令名，而不是直接执行。
func TestPaletteEnterCompletesArgCommand(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 34)
	m.input.SetValue("/")
	m.paletteIdx = 0 // /model
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if strings.TrimSpace(m.input.Value()) != "/model" {
		t.Fatalf("arg command should be completed, got %q", m.input.Value())
	}
}

// 回合进行中提交任务应排队，不能并发启动第二个 Run。
func TestBusySubmitQueuesInsteadOfStartingSecondRun(t *testing.T) {
	runs := 0
	var firstEvents chan core.Event
	m := NewModel(Config{Theme: "none", Run: func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		runs++
		ch := make(chan core.Event, 1)
		if runs == 1 {
			firstEvents = ch
		}
		ch <- core.Event{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: prompt}}
		return ch, make(chan error, 1)
	}})
	m.setSize(100, 34)
	m.input.SetValue("第一个任务")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.input.SetValue("第二个任务")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if runs != 1 || len(m.queue) != 1 {
		t.Fatalf("runs=%d queue=%d", runs, len(m.queue))
	}
	close(firstEvents)
	m.Update(streamClosedMsg{runID: m.runID})
	if runs != 2 || len(m.queue) != 0 {
		t.Fatalf("queued prompt not auto-sent: runs=%d queue=%d", runs, len(m.queue))
	}
}

// 高危审批按 y/a/n/esc 不能再返回 nil model 导致下一帧 panic。
func TestHighRiskApprovalReturnsModel(t *testing.T) {
	ch := make(chan ApprovalAsk, 1)
	m := NewModel(Config{Theme: "none", Approvals: ch})
	m.setSize(100, 40)
	m.Update(ApprovalAsk{
		Req:  core.ApprovalRequest{CallID: "c1", Tool: "shell", Command: "rm -rf /", HighRisk: true},
		Resp: make(chan core.ApprovalDecision, 1),
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if updated == nil {
		t.Fatal("Update returned nil model")
	}
	_ = updated.View()
}

// 审批卡在 80×24 / 100×30 下必须完整显示选项与输入框。
func TestApprovalCardAndComposerVisibleAtSmallSizes(t *testing.T) {
	for _, size := range [][2]int{{40, 20}, {80, 24}, {100, 30}} {
		m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
		m.setSize(size[0], size[1])
		m.Update(ApprovalAsk{
			Req:  core.ApprovalRequest{CallID: "c1", Tool: "shell", Command: "go test ./..."},
			Resp: make(chan core.ApprovalDecision, 1),
		})
		view := stripANSI.ReplaceAllString(m.View(), "")
		if !strings.Contains(view, "[y] 允许") {
			t.Fatalf("%dx%d: approval options clipped:\n%s", size[0], size[1], view)
		}
		if !strings.Contains(view, "›") {
			t.Fatalf("%dx%d: composer clipped:\n%s", size[0], size[1], view)
		}
		if n := len(strings.Split(view, "\n")); n > size[1] {
			t.Fatalf("%dx%d: view has %d lines", size[0], size[1], n)
		}
	}
}

// 审批时 Ctrl+C 必须中断，而不是被输入框吞掉。
func TestCtrlCInterruptsPendingApproval(t *testing.T) {
	cancelled := false
	ch := make(chan ApprovalAsk, 1)
	m := NewModel(Config{Theme: "none", Approvals: ch, Cancel: func() { cancelled = true }})
	m.setSize(100, 40)
	m.state = stStreaming
	m.Update(ApprovalAsk{Req: core.ApprovalRequest{Command: "go test ./..."}, Resp: make(chan core.ApprovalDecision, 1)})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !cancelled || m.pending != nil || m.state != stIdle {
		t.Fatalf("cancelled=%v pending=%v state=%d", cancelled, m.pending, m.state)
	}
}

// 帮助页里 Ctrl+C 也要有响应；忙碌时优先中断回合，不能被模态层吞掉。
func TestCtrlCWorksOnHelpPage(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 40)
	m.showHelp = true
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if m.showHelp {
		t.Fatal("ctrl+c should close help page when idle")
	}

	cancelled := false
	m = NewModel(Config{Theme: "none", Cancel: func() { cancelled = true }})
	m.setSize(100, 40)
	m.state = stStreaming
	m.showHelp = true
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !cancelled || m.showHelp || m.state != stIdle {
		t.Fatalf("ctrl+c while busy should interrupt: cancelled=%v help=%v state=%d", cancelled, m.showHelp, m.state)
	}
}

// 命令面板选中第 7 项以后高亮必须仍在窗口内。
func TestPaletteHighlightStaysVisible(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 40)
	m.input.SetValue("/")
	for i := 0; i < 8; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	panel := stripANSI.ReplaceAllString(m.renderCommandPanel(m.slashSuggestions()), "")
	if !strings.Contains(panel, "▸") || !strings.Contains(panel, "/diff") {
		t.Fatalf("highlight scrolled out:\n%s", panel)
	}
}

// 新回合必须清掉上一回合的 done 状态。
func TestNewTurnResetsTerminalState(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(100, 40)
	m.applyEvent(core.Event{Type: core.EvtTurnEnd, Data: core.TurnEndData{Terminal: "done"}})
	m.input.SetValue("下一个任务")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.term != "" {
		t.Fatalf("terminal state leaked into next turn: %q", m.term)
	}
	view := stripANSI.ReplaceAllString(m.View(), "")
	if strings.Contains(view, "· 完成") {
		t.Fatalf("stale done state visible:\n%s", view)
	}
}

// 对话必须能向上翻历史。
func TestConversationScrollsBack(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(80, 24)
	for i := 1; i <= 60; i++ {
		m.applyEvent(core.Event{Type: core.EvtNotice, Data: core.NoticeData{Level: "info", Text: fmt.Sprintf("第%02d条记录", i)}})
	}
	_ = m.View()
	if strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "第01条") {
		t.Fatal("oldest message unexpectedly visible before scrolling")
	}
	for i := 0; i < 6; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	}
	view := stripANSI.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "第01条") {
		t.Fatalf("PgUp did not scroll back:\n%s", view)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	for i := 0; i < 12; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	}
	beforeWheel := stripANSI.ReplaceAllString(m.View(), "")
	for i := 0; i < 6; i++ {
		m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	}
	if beforeWheel == stripANSI.ReplaceAllString(m.View(), "") {
		t.Fatal("mouse wheel did not scroll the conversation")
	}
}

// 成功的工具输出默认折叠，Ctrl+O 必须能展开。
func TestCtrlOTogglesToolOutput(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(100, 40)
	m.applyEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "c1", Tool: "read_file", Args: `{"path":"a.go"}`}})
	m.applyEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{CallID: "c1", OK: true, DurationMS: 12, Output: "SECRET-OUTPUT"}})
	if strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "SECRET-OUTPUT") {
		t.Fatal("successful tool output should be collapsed by default")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	if !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "SECRET-OUTPUT") {
		t.Fatal("Ctrl+O did not expand tool output")
	}
}

// theme=none 时不得再输出写死的 ANSI/256 色。
func TestThemeNoneHasNoHardcodedANSI(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(100, 40)
	raw := m.View()
	if strings.Contains(raw, "\x1b[96m") || strings.Contains(raw, "\x1b[38;5;") {
		t.Fatalf("theme=none still emits hardcoded color codes: %q", raw)
	}
}

// /sessions 必须能选中并恢复会话，而不是只打印列表。
func TestSessionPickerResumesSession(t *testing.T) {
	m := NewModel(Config{
		Theme: "none",
		ListSessions: func() ([]SessionInfo, error) {
			return []SessionInfo{{ID: "s1", Summary: "旧任务一"}, {ID: "s2", Summary: "旧任务二"}}, nil
		},
		ResumeSession: func(id string) ([]core.Event, error) {
			return []core.Event{{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "恢复的任务"}}}, nil
		},
	})
	m.setSize(100, 34)
	m.runSlash("/sessions", "/sessions", nil)
	if !m.showSessions {
		t.Fatal("session picker not opened")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.sessionID != "s2" || m.showSessions {
		t.Fatalf("sessionID=%q show=%v", m.sessionID, m.showSessions)
	}
	found := false
	for _, b := range m.blocks {
		if strings.Contains(b.content, "恢复的任务") {
			found = true
		}
	}
	if !found {
		t.Fatal("resumed events not replayed")
	}
}

// 工具 diff 只在展开时显示，且保留 +/- 原文。
func TestToolDiffShownWhenExpanded(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(100, 40)
	m.applyEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "c1", Tool: "edit_file", Args: `{"path":"a.go"}`}})
	m.applyEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{
		CallID: "c1", OK: true, DurationMS: 5, Output: "已编辑 a.go（替换 1 处）",
		Diff: "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
	}})
	if strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "+new") {
		t.Fatal("diff should be collapsed by default")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	view := stripANSI.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "+new") || !strings.Contains(view, "-old") {
		t.Fatalf("diff not shown when expanded:\n%s", view)
	}
}

// theme=none 时 Markdown 也不能输出颜色码。
func TestThemeNoneMarkdownHasNoANSI(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(100, 40)
	m.applyEvent(core.Event{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "# 标题\n\n正文 **加粗**\n"}})
	raw := m.View()
	if strings.Contains(raw, "\x1b[38;") || strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("theme=none markdown emitted color codes: %q", raw)
	}
}

// Alt+Enter 必须能输入多行，Enter 仍提交。
func TestAltEnterInsertsNewlineAndSubmits(t *testing.T) {
	var got string
	m := NewModel(Config{Theme: "none", Run: func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		got = prompt
		return make(chan core.Event), make(chan error)
	}})
	m.setSize(100, 40)
	m.input.SetValue("第一行")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m.input.InsertString("第二行")
	if !strings.Contains(m.input.Value(), "\n") {
		t.Fatalf("alt+enter did not insert newline: %q", m.input.Value())
	}
	if m.composerHeight() < 2 {
		t.Fatalf("composer height = %d", m.composerHeight())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(got, "第一行") || !strings.Contains(got, "第二行") {
		t.Fatalf("submitted prompt = %q", got)
	}
}

// 中断后旧引擎还在释放会话锁时，新任务必须先排队，不能并发抢锁。
func TestCancelDrainsBeforeStartingQueuedPrompt(t *testing.T) {
	runs := 0
	m := NewModel(Config{Theme: "none", Run: func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		runs++
		return make(chan core.Event), make(chan error)
	}})
	m.setSize(100, 40)
	m.input.SetValue("第一")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	oldID := m.runID
	m.cancel()
	if m.draining != 1 {
		t.Fatalf("draining = %d", m.draining)
	}
	m.input.SetValue("第二")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if runs != 1 || len(m.queue) != 1 {
		t.Fatalf("runs=%d queue=%d", runs, len(m.queue))
	}
	m.Update(streamClosedMsg{runID: oldID})
	if runs != 2 || m.draining != 0 || len(m.queue) != 0 {
		t.Fatalf("runs=%d draining=%d queue=%d", runs, m.draining, len(m.queue))
	}
}

// 输入历史变化要回调宿主落盘。
func TestSaveHistoryCallback(t *testing.T) {
	var saved []string
	m := NewModel(Config{Theme: "none", SaveHistory: func(items []string) { saved = append([]string{}, items...) }})
	m.setSize(100, 40)
	m.input.SetValue("记住我")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(saved) == 0 || saved[len(saved)-1] != "记住我" {
		t.Fatalf("saved = %v", saved)
	}
}

// 标题栏固定 3 行，不再容纳吉祥物/球场。
func TestBannerIsCompactTitleBar(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 40}} {
		m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
		m.setSize(size[0], size[1])
		lines := strings.Split(m.banner(), "\n")
		if len(lines) != 3 {
			t.Fatalf("%dx%d banner lines = %d", size[0], size[1], len(lines))
		}
		if strings.Contains(strings.Join(lines, ""), "⚽") {
			t.Fatalf("%dx%d banner still contains mascot/ball", size[0], size[1])
		}
	}
}

// 上下文百分比用最近一步提示词，而不是本回合累计 In+Out。
func TestContextUsesPromptTokens(t *testing.T) {
	m := NewModel(Config{Theme: "none", ContextWindow: 1000})
	m.setSize(100, 34)
	m.applyEvent(core.Event{Type: core.EvtUsage, Data: core.UsageData{TokensIn: 9000, TokensOut: 2000, PromptTokens: 750}})
	if got := m.contextPercent(); got != 25 {
		t.Fatalf("context percent = %d, want 25", got)
	}
}

// /approve 必须在计划所属会话里执行。
func TestApproveSwitchesSession(t *testing.T) {
	m := NewModel(Config{Theme: "none", SessionID: "s_current", ApprovePlan: func(id string) (string, error) {
		return "执行计划", nil
	}})
	m.setSize(100, 34)
	m.runSlash("/approve", "/approve s_plan", []string{"s_plan"})
	if m.sessionID != "s_plan" {
		t.Fatalf("sessionID = %q", m.sessionID)
	}
	found := false
	for _, b := range m.blocks {
		if b.kind == blkUser && b.content == "执行计划" {
			found = true
		}
	}
	if !found {
		t.Fatal("plan was not submitted as a turn")
	}
}

// 任何尺寸下每一行的显示宽度都不能超过终端宽度。
func TestViewNeverExceedsTerminalWidth(t *testing.T) {
	for _, size := range [][2]int{{40, 20}, {60, 20}, {80, 24}, {100, 30}, {120, 40}} {
		m := NewModel(Config{Theme: "none", Model: "deepseek-chat", Workdir: `C:\Users\dev\demo`})
		m.setSize(size[0], size[1])
		m.applyEvent(core.Event{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "任务"}})
		m.applyEvent(core.Event{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "一段比较长的中文回答，用来检查对话区是否会超出终端宽度。"}})
		m.Update(ApprovalAsk{
			Req:  core.ApprovalRequest{Tool: "shell", Command: "git push --force origin main", HighRisk: true},
			Resp: make(chan core.ApprovalDecision, 1),
		})
		check := func(label string) {
			view := stripANSI.ReplaceAllString(m.View(), "")
			for _, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > size[0] {
					t.Fatalf("%dx%d %s: line width %d > %d: %q", size[0], size[1], label, got, size[0], line)
				}
			}
		}
		check("approval")
		m.pending = nil
		m.state = stIdle
		m.input.SetValue("/")
		check("palette")
		m.input.SetValue("")
		m.showHelp = true
		check("help")
		m.showHelp = false
		m.sessionList = []SessionInfo{{ID: "s_01H8Z3A1B2C3D4E5F6G7H8J9K0", Summary: "很长的会话摘要用来检查窄屏", Terminal: "done", Modified: "2026-11-24 10:12"}}
		m.showSessions = true
		check("sessions")
		m.showSessions = false
		m.applyEvent(core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{CallID: "c1", Tool: "read_file", Args: `{"path":"a.go"}`}})
		m.applyEvent(core.Event{Type: core.EvtToolEnd, Data: core.ToolEndData{
			CallID: "c1", OK: true, Output: strings.Repeat("很长的工具输出", 20),
		}})
		m.verboseTools = true
		m.markToolsDirty()
		check("tool-output")
		m.applyEvent(core.Event{Type: core.EvtNotice, Data: core.NoticeData{Level: "warn", Text: strings.Repeat("很长的提示", 30)}})
		check("notice")
	}
}

// 宽屏分栏时，对话框 + 右面板必须正好铺满终端宽度。
func TestWideLayoutFillsWidth(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(100, 40)
	view := stripANSI.ReplaceAllString(m.View(), "")
	found := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "╭") >= 2 {
			found = true
			if got := lipgloss.Width(line); got != 100 {
				t.Fatalf("dialog+panel width = %d, want 100: %q", got, line)
			}
		}
	}
	if !found {
		t.Fatal("no split dialog+panel row found")
	}
}

// Markdown 增量渲染只在完整段落处切分，代码块内不切。
func TestStableMarkdownPrefix(t *testing.T) {
	if got, want := stableMarkdownPrefix("# 标题\n\n正文"), len("# 标题\n\n"); got != want {
		t.Fatalf("prefix = %d, want %d", got, want)
	}
	if got := stableMarkdownPrefix("```\ncode\n\nmore"); got != 0 {
		t.Fatalf("open code fence should not be split, got %d", got)
	}
}

// 大段粘贴折叠为占位符，提交时还原完整内容。
func TestLargePasteFoldsAndRestores(t *testing.T) {
	var got string
	m := NewModel(Config{Theme: "none", Run: func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		got = prompt
		return make(chan core.Event), make(chan error)
	}})
	m.setSize(100, 40)
	big := strings.Repeat("a", 6000)
	m.Update(tea.KeyMsg{Paste: true, Runes: []rune(big)})
	if !strings.Contains(m.input.Value(), "[粘贴") {
		t.Fatalf("paste not folded: %q", m.input.Value())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got != big {
		t.Fatalf("submitted paste length = %d, want %d", len(got), len(big))
	}
}

// Ctrl+R 只读重放当前会话。
func TestCtrlRReplaysCurrentSession(t *testing.T) {
	m := NewModel(Config{Theme: "none", SessionID: "s_replay", ResumeSession: func(id string) ([]core.Event, error) {
		return []core.Event{{Type: core.EvtTurnStart, Data: core.TurnStartData{Input: "重放任务"}}}, nil
	}})
	m.setSize(100, 40)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	found := false
	for _, b := range m.blocks {
		if b.content == "重放任务" {
			found = true
		}
	}
	if !found || !strings.Contains(m.hint, "重放") {
		t.Fatalf("replay missing: blocks=%d hint=%q", len(m.blocks), m.hint)
	}
}

// Ctrl+Home / Ctrl+End 跳到顶/底。
func TestCtrlHomeEndScroll(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(80, 24)
	for i := 1; i <= 60; i++ {
		m.applyEvent(core.Event{Type: core.EvtNotice, Data: core.NoticeData{Level: "info", Text: fmt.Sprintf("第%02d条", i)}})
	}
	_ = m.View()
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	if !m.view.AtTop() {
		t.Fatal("ctrl+home did not reach top")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	if !m.view.AtBottom() {
		t.Fatal("ctrl+end did not reach bottom")
	}
}

// 普通审批禁用输入；高危审批必须保留 yes 输入；结束后恢复焦点。
func TestApprovalFocusPolicy(t *testing.T) {
	m := NewModel(Config{Theme: "none"})
	m.setSize(100, 40)
	m.Update(ApprovalAsk{Req: core.ApprovalRequest{Command: "go test ./..."}, Resp: make(chan core.ApprovalDecision, 1)})
	if m.input.Focused() {
		t.Fatal("normal approval input should be blurred")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.input.Focused() {
		t.Fatal("input should regain focus after approval")
	}

	m.Update(ApprovalAsk{Req: core.ApprovalRequest{Command: "rm -rf /", HighRisk: true}, Resp: make(chan core.ApprovalDecision, 1)})
	if !m.input.Focused() {
		t.Fatal("high-risk approval needs focused input for yes")
	}
}

// 状态栏显示本轮证据数与会话 ID。
func TestStatusBarShowsEvidenceAndSession(t *testing.T) {
	m := NewModel(Config{Theme: "none", Model: "m", Workdir: "p"})
	m.setSize(110, 40)
	m.sessionID = "s_0123456789abcdef"
	m.applyEvent(core.Event{Type: core.EvtGateAllow, Data: core.GateAllowData{EvidenceIDs: []string{"e1", "e2"}}})
	view := stripANSI.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "证据 2") || !strings.Contains(view, "会话 s_01234567") {
		t.Fatalf("status bar missing evidence/session:\n%s", view)
	}
}

// 回合必须把当前会话 ID 传给引擎，才能续接上下文。
func TestRunReceivesSessionID(t *testing.T) {
	var got string
	m := NewModel(Config{Theme: "none", SessionID: "s_existing", Run: func(prompt, sessionID string) (<-chan core.Event, <-chan error) {
		got = sessionID
		return make(chan core.Event), make(chan error)
	}})
	m.setSize(100, 34)
	m.input.SetValue("继续")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got != "s_existing" {
		t.Fatalf("session id = %q", got)
	}
}
