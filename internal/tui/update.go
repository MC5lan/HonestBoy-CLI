package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case ApprovalAsk:
		if m.pending != nil {
			m.denyPending("被新的审批请求覆盖")
		}
		m.pending = &msg
		m.state = stApproving
		m.showHelp = false
		m.showSessions = false
		m.input.SetValue("")
		m.input.SetHeight(1)
		m.hint = ""
		m.refreshView()
		return m, waitForApproval(m.cfg.Approvals)

	case approvalClosedMsg:
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.view.LineUp(3)
				m.follow = m.view.AtBottom()
			case tea.MouseButtonWheelDown:
				m.view.LineDown(3)
				m.follow = m.view.AtBottom()
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case eventMsg:
		if m.run == nil || msg.runID != m.run.id {
			// 过期回合：继续排空 channel，避免引擎 goroutine 阻塞在发送上。
			return m, waitForEvent(msg.runID, msg.ch)
		}
		m.applyEvent(msg.ev)
		return m, waitForEvent(msg.runID, msg.ch)

	case streamClosedMsg:
		return m, m.onStreamClosed(msg.runID)

	case spinnerTickMsg:
		if m.busy() {
			m.spinnerIdx++
			return m, spinnerTick()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Ctrl+C 在任何状态下都必须能中断或退出。
	if key == "ctrl+c" {
		return m, m.cancel()
	}
	// 审批是当前最高优先级的交互；Esc 在审批里表示拒绝，而不是取消整个回合。
	if m.pending != nil {
		if cmd, handled := m.handleApprovalKey(msg); handled {
			return m, cmd
		}
	}
	// 忙碌时 Esc 的语义也是中断，帮助页不能把它吞掉。
	if key == "esc" && m.busy() {
		return m, m.cancel()
	}

	if m.showHelp {
		switch key {
		case "?", "esc", "enter", "q":
			m.showHelp = false
			m.hint = ""
			m.refreshView()
		case "pgup":
			m.helpOffset -= 8
		case "pgdown":
			m.helpOffset += 8
		case "ctrl+u":
			m.helpOffset -= 4
		case "ctrl+d":
			m.helpOffset += 4
		}
		if m.helpOffset < 0 {
			m.helpOffset = 0
		}
		return m, nil
	}

	if m.showSessions {
		switch key {
		case "up", "k":
			if m.sessionIdx > 0 {
				m.sessionIdx--
			}
		case "down", "j":
			if m.sessionIdx < len(m.sessionList)-1 {
				m.sessionIdx++
			}
		case "enter":
			return m, m.resumeSelectedSession()
		case "esc", "q":
			m.showSessions = false
			m.hint = ""
			m.refreshView()
		}
		return m, nil
	}

	switch key {
	case "pgup":
		m.view.PageUp()
		m.follow = m.view.AtBottom()
		return m, nil
	case "pgdown":
		m.view.PageDown()
		m.follow = m.view.AtBottom()
		return m, nil
	case "ctrl+u":
		m.view.HalfPageUp()
		m.follow = m.view.AtBottom()
		return m, nil
	case "ctrl+d":
		m.view.HalfPageDown()
		m.follow = m.view.AtBottom()
		return m, nil
	case "ctrl+o":
		m.verboseTools = !m.verboseTools
		m.markToolsDirty()
		return m, nil
	case "ctrl+l":
		m.blocks = nil
		m.term = ""
		m.err = nil
		m.hint = ""
		m.refreshView()
		return m, nil
	case "?":
		if strings.TrimSpace(m.input.Value()) == "" {
			m.showHelp = true
			m.helpOffset = 0
			m.hint = ""
			return m, nil
		}
	case "enter":
		if m.slashPanelVisible() {
			return m, m.submitPalette()
		}
		return m, m.submit()
	case "alt+enter":
		m.input.InsertString("\n")
		m.resizeComposer()
		return m, nil
	case "esc":
		if m.busy() {
			return m, m.cancel()
		}
		if m.slashPanelVisible() {
			m.input.SetValue("")
			m.input.SetHeight(1)
			return m, nil
		}
		m.hint = ""
		return m, nil
	case "up":
		if m.slashPanelVisible() {
			m.movePalette(-1)
			return m, nil
		}
		if m.historyPrev() {
			return m, nil
		}
	case "down":
		if m.slashPanelVisible() {
			m.movePalette(1)
			return m, nil
		}
		if m.historyNext() {
			return m, nil
		}
	case "tab":
		if m.slashPanelVisible() {
			m.completePalette()
			return m, nil
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	// 输入变化后命令列表与选中位置都可能变；下一次打开面板从头开始。
	m.paletteIdx, m.paletteOffset = 0, 0
	m.resizeComposer()
	return m, cmd
}

func (m *Model) onStreamClosed(runID uint64) tea.Cmd {
	if m.run == nil || runID != m.run.id {
		if m.draining > 0 {
			m.draining--
		}
		return m.startNextQueued()
	}
	errs := m.run.errs
	m.run = nil
	m.state = stIdle
	// 引擎已结束，未回答的审批不会再有人消费，直接收起卡片。
	m.pending = nil
	if errs != nil {
		select {
		case err, ok := <-errs:
			if ok && err != nil {
				m.err = err
				m.blocks = append(m.blocks, block{kind: blkError, content: err.Error(), dirty: true})
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	m.refreshView()
	return m.startNextQueued()
}

func (m *Model) startNextQueued() tea.Cmd {
	if m.draining > 0 || len(m.queue) == 0 {
		return nil
	}
	next := m.queue[0]
	m.queue = m.queue[1:]
	m.hint = fmt.Sprintf("自动发送排队任务（剩余 %d 条）", len(m.queue))
	return m.startPrompt(next)
}

func (m *Model) resizeComposer() {
	lines := m.input.LineCount()
	if lines < 1 {
		lines = 1
	}
	if lines > 6 {
		lines = 6
	}
	m.input.SetHeight(lines)
}

func (m *Model) historyPrev() bool {
	if len(m.history) == 0 || m.input.LineCount() > 1 {
		return false
	}
	if m.histIdx > 0 {
		m.histIdx--
		m.input.SetValue(m.history[m.histIdx])
		m.input.CursorEnd()
		m.resizeComposer()
		return true
	}
	return false
}

func (m *Model) historyNext() bool {
	if len(m.history) == 0 || m.input.LineCount() > 1 {
		return false
	}
	if m.histIdx < len(m.history)-1 {
		m.histIdx++
		m.input.SetValue(m.history[m.histIdx])
	} else {
		m.histIdx = len(m.history)
		m.input.SetValue("")
	}
	m.input.CursorEnd()
	m.resizeComposer()
	return true
}

func (m *Model) applyEvent(ev core.Event) {
	if ev.SessionID != "" {
		m.sessionID = ev.SessionID
	}
	switch ev.Type {
	case core.EvtTurnStart:
		// 回放时每个 turn_start 都重置“本轮”，最终只保留最后一个回合的统计。
		m.resetTurnStats()
		if d, ok := ev.Data.(core.TurnStartData); ok {
			n := len(m.blocks)
			if n == 0 || m.blocks[n-1].kind != blkUser || m.blocks[n-1].content != d.Input {
				m.blocks = append(m.blocks, block{kind: blkUser, content: d.Input, dirty: true})
			}
			m.refreshView()
		}
	case core.EvtTextDelta:
		d, ok := ev.Data.(core.TextDeltaData)
		if !ok || d.Text == "" {
			return
		}
		idx := len(m.blocks) - 1
		if idx < 0 || m.blocks[idx].kind != blkAssistant {
			m.blocks = append(m.blocks, block{kind: blkAssistant, dirty: true})
			idx = len(m.blocks) - 1
		}
		m.blocks[idx].content += d.Text
		m.blocks[idx].dirty = true
		if time.Since(m.lastRender) >= 16*time.Millisecond || strings.Contains(d.Text, "\n") {
			m.lastRender = time.Now()
			m.refreshView()
		}
	case core.EvtToolRequest:
		d, ok := ev.Data.(core.ToolRequestData)
		if !ok {
			return
		}
		m.blocks = append(m.blocks, block{
			kind: blkTool, collapsed: true, dirty: true, ok: true,
			meta: map[string]string{"call": d.CallID, "tool": core.ToolLabel(d.Tool), "args": d.Args},
		})
		m.refreshView()
	case core.EvtToolStart:
		m.state = stToolRunning
	case core.EvtToolEnd:
		d, ok := ev.Data.(core.ToolEndData)
		if !ok {
			return
		}
		m.state = stStreaming
		if d.OK {
			m.turnToolsOK++
			m.sessToolsOK++
		} else {
			m.turnToolsFail++
			m.sessToolsFail++
		}
		for i := len(m.blocks) - 1; i >= 0; i-- {
			b := &m.blocks[i]
			if b.kind == blkTool && b.meta["call"] == d.CallID {
				b.content = d.Output
				b.ok = d.OK
				b.duration = d.DurationMS
				b.collapsed = d.OK
				b.dirty = true
				if d.Diff != "" {
					b.meta["diff"] = d.Diff
				}
				break
			}
		}
		m.refreshView()
	case core.EvtToolDenied:
		d, ok := ev.Data.(core.ToolDeniedData)
		if !ok {
			return
		}
		for i := len(m.blocks) - 1; i >= 0; i-- {
			b := &m.blocks[i]
			if b.kind == blkTool && b.meta["call"] == d.CallID {
				b.content, b.ok, b.dirty = d.Reason, false, true
				b.collapsed = false
				break
			}
		}
		m.refreshView()
	case core.EvtGateAllow:
		m.turnGateAllow++
		m.sessGateAllow++
		m.blocks = append(m.blocks, block{kind: blkGateAllow, dirty: true})
		m.refreshView()
	case core.EvtGateDeny:
		d, ok := ev.Data.(core.GateDenyData)
		if !ok {
			return
		}
		m.turnGateDeny++
		m.sessGateDeny++
		m.obsRule = d.Rule
		m.obsReason = d.Reason
		m.blocks = append(m.blocks, block{
			kind: blkGateDeny, content: d.Reason, dirty: true,
			meta: map[string]string{"rule": d.Rule},
		})
		m.refreshView()
	case core.EvtAbstain:
		d, ok := ev.Data.(core.AbstainData)
		if !ok {
			return
		}
		m.blocks = append(m.blocks, block{kind: blkAbstain, content: d.Reason, dirty: true})
		m.refreshView()
	case core.EvtNotice:
		d, ok := ev.Data.(core.NoticeData)
		if !ok {
			return
		}
		m.blocks = append(m.blocks, block{kind: blkNotice, content: d.Text, dirty: true,
			meta: map[string]string{"level": d.Level}})
		m.refreshView()
	case core.EvtDiffReview:
		if d, ok := ev.Data.(core.DiffReviewData); ok {
			stat := d.Stat
			if len([]rune(stat)) > 120 {
				stat = string([]rune(stat)[:120]) + "…"
			}
			m.blocks = append(m.blocks, block{kind: blkEvidence,
				content: "自动提交 " + d.Commit + "（" + fmt.Sprint(len(d.Files)) + " 文件）" + stat,
				dirty:   true})
			m.refreshView()
		}
	case core.EvtUsage:
		if d, ok := ev.Data.(core.UsageData); ok {
			m.tokensIn, m.tokensOut = int(d.TokensIn), int(d.TokensOut)
			if d.PromptTokens > 0 {
				m.promptTokens = int(d.PromptTokens)
			}
		}
	case core.EvtError:
		d, ok := ev.Data.(core.ErrorData)
		if !ok {
			return
		}
		m.blocks = append(m.blocks, block{kind: blkError, content: d.Message, dirty: true})
		m.refreshView()
	case core.EvtTurnEnd:
		if d, ok := ev.Data.(core.TurnEndData); ok {
			m.term = core.Terminal(d.Terminal)
			m.blocks = append(m.blocks, block{
				kind:    blkTurnEnd,
				content: m.term.String() + " · " + (time.Duration(d.DurationMS) * time.Millisecond).String(),
				dirty:   true,
			})
		}
		m.state = stIdle
		m.refreshView()
	}
}
