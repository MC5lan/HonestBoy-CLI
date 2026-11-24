package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case ApprovalAsk:
		m.pending = &msg
		m.state = stApproving
		m.hint = ""
		m.refreshView()
		return m, waitForApproval(m.cfg.Approvals)

	case approvalClosedMsg:
		return m, nil

	case tea.KeyMsg:
		// 帮助页是模态层：先处理关闭键，其余按键不穿透到输入框。
		if m.showHelp {
			switch msg.String() {
			case "?", "esc", "enter", "q":
				m.showHelp = false
				m.hint = ""
				m.refreshView()
			}
			return m, nil
		}
		if m.state == stApproving && m.pending != nil {
			if m.pending.Req.HighRisk {
				switch msg.String() {
				case "y", "a", "n", "esc":
					m.replyDenyHighRisk()
					return nil, nil
				case "enter":
					if strings.TrimSpace(m.input.Value()) == "yes" {
						m.input.SetValue("")
						return m, m.respondApproval(true, false)
					}
				}
				var hcmd tea.Cmd
				m.input, hcmd = m.input.Update(msg)
				return m, hcmd
			}
			switch msg.String() {
			case "y":
				return m, m.respondApproval(true, false)
			case "a":
				return m, m.respondApproval(true, true)
			case "n", "esc":
				return m, m.respondApproval(false, false)
			case "enter":
				if strings.TrimSpace(m.input.Value()) == "yes" {
					m.input.SetValue("")
					return m, m.respondApproval(true, false)
				}
			}
			var acmd tea.Cmd
			m.input, acmd = m.input.Update(msg)
			return m, acmd
		}
		switch msg.String() {
		case "?":
			if strings.TrimSpace(m.input.Value()) == "" {
				m.showHelp = true
				return m, nil
			}
		case "ctrl+c":
			return m, m.cancel()
		case "ctrl+l":
			m.blocks = nil
			m.refreshView()
			return m, nil
		case "enter":
			return m, m.submit()
		case "esc":
			// Codex 行为：忙碌时 esc 中断当前回合；空闲时 esc 不退出。
			if m.state == stStreaming || m.state == stToolRunning {
				return m, m.cancel()
			}
			m.hint = ""
			return m, nil
		case "up":
			// 命令面板可见时，上下键在面板内选择；否则翻历史。
			if sug := m.slashSuggestions(); len(sug) > 0 {
				if m.cmdIdx > 0 {
					m.cmdIdx--
				}
				return m, nil
			}
			if m.histIdx > 0 {
				m.histIdx--
				m.input.SetValue(m.history[m.histIdx])
			}
			return m, nil
		case "down":
			if sug := m.slashSuggestions(); len(sug) > 0 {
				if m.cmdIdx < len(sug)-1 {
					m.cmdIdx++
				}
				return m, nil
			}
			if m.histIdx < len(m.history)-1 {
				m.histIdx++
				m.input.SetValue(m.history[m.histIdx])
			} else {
				m.histIdx = len(m.history)
				m.input.SetValue("")
			}
			return m, nil
		case "tab":
			if sug := m.slashSuggestions(); len(sug) > 0 {
				idx := m.cmdIdx
				if idx >= len(sug) {
					idx = 0
				}
				m.input.SetValue(sug[idx].name + " ")
				m.input.CursorEnd()
				m.cmdIdx = 0
				return m, nil
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.cmdIdx = 0 // 输入变化后重置命令面板选择
		return m, cmd

	case eventMsg:
		m.applyEvent(core.Event(msg))
		return m, waitForEvent(m.events)

	case streamClosedMsg:
		m.state = stIdle
		m.interruptSeen = false
		if m.errCh != nil {
			select {
			case err, ok := <-m.errCh:
				if ok && err != nil {
					m.err = err
					m.blocks = append(m.blocks, block{kind: blkError, content: err.Error(), dirty: true})
				}
			case <-time.After(50 * time.Millisecond):
			}
		}
		m.refreshView()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case spinnerTickMsg:
		m.pitchTick++
		// 射门动画：kickFrame 从 1 递增到超过 7（球飞到右门）后停在 goal。
		if m.kickFrame > 0 && m.kickFrame < 8 {
			m.kickFrame++
		}
		kicking := m.kickFrame > 0 && m.kickFrame < 8
		if m.state == stStreaming || m.state == stToolRunning || kicking {
			m.spinnerIdx++
			return m, spinnerTick()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) replyDenyHighRisk() {
	if m.pending == nil {
		return
	}
	select {
	case m.pending.Resp <- core.ApprovalDecision{Allowed: false, Reason: "高危命令需输入 yes 确认"}:
	default:
	}
	m.pending = nil
	m.state = stStreaming
	m.hint = "已拒绝高危命令"
	m.refreshView()
}

func (m *Model) respondApproval(allowed, all bool) tea.Cmd {
	if m.pending == nil {
		return nil
	}
	d := core.ApprovalDecision{Allowed: allowed, AllowAll: all}
	if !allowed {
		d.Reason = "用户拒绝执行"
	}
	select {
	case m.pending.Resp <- d:
	default:
	}
	m.pending = nil
	m.state = stStreaming
	if allowed {
		m.hint = "已批准"
	} else {
		m.hint = "已拒绝"
	}
	m.refreshView()
	return nil
}

func (m *Model) applyEvent(ev core.Event) {
	switch ev.Type {
	case core.EvtTurnStart:
		if d, ok := ev.Data.(core.TurnStartData); ok {
			// submit() 已经先显示过用户输入；事件回放时才需要补一条。
			// 连续相同的用户块不重复添加，避免“> 你好”出现两次。
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
		// 只延续“最后一个块”上的助手文本；中间插入工具/通知后，
		// 后续文本要新起一块，保证渲染顺序与事件顺序一致。
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
			m.obsToolsOK++
		} else {
			m.obsToolsFail++
		}
		for i := len(m.blocks) - 1; i >= 0; i-- {
			b := &m.blocks[i]
			if b.kind == blkTool && b.meta["call"] == d.CallID {
				b.content = d.Output
				b.ok = d.OK
				b.duration = d.DurationMS
				b.collapsed = d.OK
				b.dirty = true
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
		m.obsAllow++
		m.blocks = append(m.blocks, block{kind: blkGateAllow, dirty: true})
		m.startKick() // 检查通过 → 一脚踢向右门
		m.refreshView()
	case core.EvtGateDeny:
		d, ok := ev.Data.(core.GateDenyData)
		if !ok {
			return
		}
		m.obsDeny++
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
		m.interruptSeen = false
		m.refreshView()
	}
}

// refreshView 只渲染 dirty 块，其余用缓存；视口在底部时自动跟随。
func (m *Model) refreshView() {
	width := m.width - 4
	if width < 20 {
		width = 20
	}
	var sb strings.Builder
	for i := range m.blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(m.blocks[i].Render(m.renderer, width, m.th))
	}
	m.view.SetContent(sb.String())
	m.view.GotoBottom()
}
