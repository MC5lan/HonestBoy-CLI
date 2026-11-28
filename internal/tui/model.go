package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/theme"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

var errNoRunner = errors.New("TUI 未装配运行回调")

type state uint8

const (
	stIdle state = iota
	stStreaming
	stToolRunning
	stApproving
)

// Settings 是 /reload 可安全热更新的配置子集；工作区不在此列，避免沙箱错位。
type Settings struct {
	Model    string
	Protocol string
	BaseURL  string
	Theme    string
}

// SessionInfo 是会话选择器需要的精简信息；由 cmd 层注入，tui 不依赖 store。
type SessionInfo struct {
	ID       string
	Summary  string
	Terminal string
	Modified string
}

// Config 由 cmd 层装配；tui 不直接依赖 engine/tools/store，避免依赖方向违规。
type Config struct {
	Workdir  string
	Model    string
	Theme    string
	History  []string
	Protocol string
	BaseURL  string
	// Run 启动一回合。sessionID 为空表示新会话；非空时引擎续接同一会话。
	Run           func(prompt, sessionID string) (<-chan core.Event, <-chan error)
	Cancel        func()
	Approvals     <-chan ApprovalAsk
	Replay        []core.Event
	SessionID     string
	ContextWindow int
	SetModel      func(string) error
	// Exec 执行需要访问 store/engine/git 的无状态命令（verify/stats/diff/undo 等）。
	Exec func(cmd string, args []string, sessionID string) (string, error)
	// ListSessions / ResumeSession 支撑 TUI 内会话切换。
	ListSessions  func() ([]SessionInfo, error)
	ResumeSession func(id string) ([]core.Event, error)
	// ApprovePlan 读取计划模式的 Receipt 并返回可执行的计划文本。
	ApprovePlan func(id string) (string, error)
	// Reload 读取配置的可热更新子集；SaveTheme 持久化主题。
	Reload    func() (Settings, error)
	SaveTheme func(string) error
	// Compact 请求下一回合压缩历史；由宿主决定具体预算。
	Compact func()
	// SaveHistory 在输入历史变化时落盘，供下次启动恢复。
	SaveHistory func([]string)
	Onboard     bool
}

type Model struct {
	width, height int
	cfg           Config
	state         state

	blocks       []block
	textFilter   core.VisibleTextFilter
	controlCalls map[string]bool
	view         viewport.Model
	follow       bool
	newBelow     bool
	verboseTools bool

	input         textarea.Model
	history       []string
	histIdx       int
	paletteIdx    int
	paletteOffset int
	sessionList   []SessionInfo
	sessionIdx    int
	showSessions  bool

	spinnerIdx int

	run        *runState
	runID      uint64
	queue      []string
	draining   int
	execBusy   bool
	pasteText  string
	pasteToken string

	hint       string
	err        error
	term       core.Terminal
	startedAt  time.Time
	lastRender time.Time
	renderer   *render.Renderer
	th         theme.Theme
	ready      bool
	showHelp   bool
	helpOffset int

	sessionID    string
	tokensIn     int
	tokensOut    int
	promptTokens int
	ctxSystem    int
	ctxHB        int
	ctxHistory   int
	ctxTools     int

	turnToolsOK      int
	turnToolsFail    int
	turnGateAllow    int
	turnGateDeny     int
	turnEvidence     int
	approvalAt       time.Time
	approvalExpanded bool
	lastCtrlC        time.Time
	sessToolsOK      int
	sessToolsFail    int
	sessGateAllow    int
	sessGateDeny     int
	obsRule          string
	obsReason        string

	pending *ApprovalAsk
}

func NewModel(cfg Config) *Model {
	in := textarea.New()
	in.Prompt = "› "
	in.Placeholder = "输入任务…（Alt+Enter 换行）"
	in.ShowLineNumbers = false
	in.CharLimit = 0
	in.SetWidth(76)
	in.SetHeight(1)
	in.Focus()

	th := theme.Load(cfg.Theme)
	m := &Model{
		cfg: cfg, input: in, view: viewport.New(80, 20),
		th: th, renderer: render.New(80, th.Name),
		history: append([]string{}, cfg.History...), follow: true,
		sessionID: cfg.SessionID,
	}
	m.histIdx = len(m.history)
	for _, ev := range cfg.Replay {
		m.applyEvent(ev)
	}
	if n := len(cfg.Replay); n > 0 && cfg.Replay[n-1].Type == core.EvtTurnStart {
		m.hint = "上次会话中断，可继续输入或 /new 开始新会话"
	}
	if cfg.Onboard {
		m.hint = "首次使用：hb init 配置端点与密钥 · / 看命令 · ? 看快捷键"
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, waitForApproval(m.cfg.Approvals))
}

func (m *Model) busy() bool {
	return m.state != stIdle || m.run != nil || m.draining > 0 || m.execBusy
}

// submit 处理一次回车：斜杠命令立即执行；普通任务在忙碌时进入队列。
func (m *Model) submit() tea.Cmd {
	prompt := strings.TrimSpace(m.expandPaste(m.input.Value()))
	if prompt == "" {
		return nil
	}
	if name, args, ok := SlashCommand(prompt); ok {
		m.input.SetValue("")
		return m.runSlash(name, prompt, args)
	}
	m.remember(prompt)
	if m.busy() {
		m.queue = append(m.queue, prompt)
		m.input.SetValue("")
		m.input.SetHeight(1)
		m.hint = fmt.Sprintf("执行中，已排队 %d 条 · 本回合结束后自动发送 · Esc 清空", len(m.queue))
		m.refreshView()
		return nil
	}
	return m.startPrompt(prompt)
}

// startPrompt 真正启动一回合；调用方负责去重与入队判断。
func (m *Model) startPrompt(prompt string) tea.Cmd {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}
	m.input.SetValue("")
	m.input.SetHeight(1)
	m.histIdx = len(m.history)
	m.paletteIdx, m.paletteOffset = 0, 0
	m.cfg.Onboard = false
	m.blocks = append(m.blocks, block{kind: blkUser, content: prompt, dirty: true})
	m.state = stStreaming
	m.startedAt = time.Now()
	m.term = ""
	m.err = nil
	m.hint = ""
	m.newBelow = false
	m.follow = true
	m.resetTurnStats()
	if m.cfg.Run == nil {
		m.err = errNoRunner
		m.state = stIdle
		return nil
	}
	m.runID++
	events, errs := m.cfg.Run(prompt, m.sessionID)
	if events == nil {
		m.err = errNoRunner
		m.state = stIdle
		return nil
	}
	m.run = &runState{id: m.runID, errs: errs}
	m.refreshView()
	return tea.Batch(waitForEvent(m.runID, events), spinnerTick())
}

func (m *Model) expandPaste(s string) string {
	if m.pasteToken != "" && strings.Contains(s, m.pasteToken) {
		s = strings.ReplaceAll(s, m.pasteToken, m.pasteText)
	}
	m.pasteText, m.pasteToken = "", ""
	return s
}

// replayCurrent 只读重放当前会话，不启动模型、不写入会话。
func (m *Model) replayCurrent() tea.Cmd {
	if m.busy() {
		m.hint = "执行中不能重放；先 Esc 中断"
		return nil
	}
	if m.sessionID == "" {
		m.hint = "当前还没有会话可重放"
		return nil
	}
	if m.cfg.ResumeSession == nil {
		m.hint = "宿主未注入会话重放能力"
		return nil
	}
	events, err := m.cfg.ResumeSession(m.sessionID)
	if err != nil {
		m.hint = "重放失败: " + err.Error()
		return nil
	}
	m.blocks = nil
	m.term = ""
	m.err = nil
	m.showHelp = false
	m.showSessions = false
	m.resetTurnStats()
	m.resetSessionStats()
	for _, ev := range events {
		m.applyEvent(ev)
	}
	m.follow = true
	m.hint = "已只读重放当前会话（未写入磁盘）"
	m.refreshView()
	return nil
}

func (m *Model) remember(prompt string) {
	if n := len(m.history); n == 0 || m.history[n-1] != prompt {
		m.history = append(m.history, prompt)
		if len(m.history) > 200 {
			m.history = m.history[len(m.history)-200:]
		}
		if m.cfg.SaveHistory != nil {
			m.cfg.SaveHistory(append([]string{}, m.history...))
		}
	}
	m.histIdx = len(m.history)
}

func (m *Model) runSlash(name, line string, args []string) tea.Cmd {
	switch name {
	case "/quit":
		return tea.Quit
	case "/model":
		if len(args) == 0 {
			m.hint = "当前模型: " + m.cfg.Model + " · 用法: /model <名称>，如 /model deepseek-reasoner"
			return nil
		}
		next := strings.TrimSpace(args[0])
		if m.cfg.SetModel != nil {
			if err := m.cfg.SetModel(next); err != nil {
				m.hint = "切换失败: " + err.Error()
				return nil
			}
		}
		m.cfg.Model = next
		m.hint = "模型已切换为 " + next + "（下一回合生效）"
	case "/config":
		proto := m.cfg.Protocol
		if proto == "" {
			proto = "openai-chat"
		}
		base := m.cfg.BaseURL
		if base == "" {
			base = "（未设置）"
		}
		m.hint = "模型 " + m.cfg.Model + " · 协议 " + proto + " · 端点 " + base
	case "/path":
		m.hint = "工作区: " + m.cfg.Workdir
	case "/clear":
		m.blocks = nil
		m.term = ""
		m.err = nil
		m.pasteText, m.pasteToken = "", ""
		m.hint = "已清空当前会话显示（会话记录仍在磁盘）"
	case "/new", "/reset":
		if m.busy() {
			m.hint = "执行中不能切换会话；先 Esc 中断"
			return nil
		}
		m.blocks = nil
		m.sessionID = ""
		m.term = ""
		m.err = nil
		m.pasteText, m.pasteToken = "", ""
		m.resetTurnStats()
		m.resetSessionStats()
		m.hint = "已开始新会话；下一次输入会创建新的会话文件"
	case "/help":
		m.showHelp = true
		m.helpOffset = 0
		m.hint = ""
	case "/setup":
		m.hint = "配置向导：退出 TUI 后运行 hb init"
	case "/context":
		window := m.cfg.ContextWindow
		if window <= 0 {
			window = 960000
		}
		m.hint = fmt.Sprintf("上下文 ~%d%% · 最近提示 %d/%d · 系统 %d · HB.md %d · 历史 %d · 工具 %d（估算）",
			m.contextPercent(), m.promptTokens, window, m.ctxSystem, m.ctxHB, m.ctxHistory, m.ctxTools)
	case "/compact":
		if m.cfg.Compact == nil {
			m.hint = "上下文会在下一回合自动按预算压缩（宿主未注入手动压缩）"
			return nil
		}
		m.cfg.Compact()
		m.hint = "已请求压缩：下一回合使用更短历史（旧工具输出截断、旧对话丢弃）"
	case "/theme":
		if len(args) == 0 {
			m.hint = "当前主题: " + m.th.Name + " · 用法: /theme dark|light|none|auto"
			return nil
		}
		name := strings.ToLower(strings.TrimSpace(args[0]))
		switch name {
		case "dark", "light", "none", "auto":
		default:
			m.hint = "未知主题 " + name + "；可选 dark / light / none / auto"
			return nil
		}
		if m.cfg.SaveTheme != nil {
			if err := m.cfg.SaveTheme(name); err != nil {
				m.hint = "保存主题失败: " + err.Error()
				return nil
			}
		}
		m.applyTheme(name)
		m.hint = "主题已切换为 " + m.th.Name
	case "/reload":
		if m.cfg.Reload == nil {
			m.hint = "宿主未注入配置重载能力"
			return nil
		}
		s, err := m.cfg.Reload()
		if err != nil {
			m.hint = "重载失败: " + err.Error()
			return nil
		}
		m.applySettings(s)
		m.hint = "配置已重载（模型/协议/端点/主题；工作区仍为当前会话）"
	case "/verbose":
		return m.verboseCommand(args)
	case "/sessions":
		return m.openSessionPicker()
	case "/approve":
		if m.busy() {
			m.hint = "执行中不能批准计划；先 Esc 中断"
			return nil
		}
		if len(args) == 0 {
			m.hint = "用法: /approve <session-id>（先 hb run --plan）"
			return nil
		}
		if m.cfg.ApprovePlan == nil {
			m.hint = "宿主未注入计划批准能力"
			return nil
		}
		plan, err := m.cfg.ApprovePlan(args[0])
		if err != nil {
			m.hint = "批准失败: " + err.Error()
			return nil
		}
		if strings.TrimSpace(plan) == "" {
			m.hint = "该会话没有待批准的计划"
			return nil
		}
		if m.sessionID != args[0] {
			// 计划属于另一个会话：清空显示，避免两段会话的块混在一起。
			m.blocks = nil
			m.term = ""
			m.resetTurnStats()
			m.resetSessionStats()
		}
		m.sessionID = args[0]
		return m.startPrompt(plan)
	case "/verify", "/stats", "/diff", "/undo", "/rewind":
		if m.execBusy {
			m.hint = "上一条命令还在执行，稍等…"
			return nil
		}
		if (name == "/undo" || name == "/rewind") && (m.state != stIdle || m.run != nil || m.draining > 0) {
			m.hint = "执行中不能修改代码；先 Esc 中断"
			return nil
		}
		if m.cfg.Exec == nil {
			m.hint = "该命令需要宿主注入执行器"
			return nil
		}
		exec := m.cfg.Exec
		cmdName := name
		argsCopy := append([]string{}, args...)
		sessionID := m.sessionID
		m.execBusy = true
		m.hint = "正在执行 " + name + "…"
		m.refreshView()
		return func() tea.Msg {
			out, err := exec(cmdName, argsCopy, sessionID)
			return execResultMsg{cmd: cmdName, out: out, err: err}
		}
	default:
		m.hint = "未知命令 " + name + "；相近: " + strings.Join(suggestCommands(name), ", ")
	}
	return nil
}

func (m *Model) openSessionPicker() tea.Cmd {
	if m.busy() {
		m.hint = "执行中不能切换会话；先 Esc 中断"
		return nil
	}
	if m.cfg.ListSessions == nil {
		m.hint = "宿主未注入会话列表能力"
		return nil
	}
	list, err := m.cfg.ListSessions()
	if err != nil {
		m.hint = "读取会话失败: " + err.Error()
		return nil
	}
	if len(list) == 0 {
		m.hint = "暂无会话"
		return nil
	}
	m.sessionList = list
	m.sessionIdx = 0
	for i := range list {
		if list[i].ID == m.sessionID {
			m.sessionIdx = i
			break
		}
	}
	m.showSessions = true
	m.showHelp = false
	m.hint = ""
	m.refreshView()
	return nil
}

func (m *Model) resumeSelectedSession() tea.Cmd {
	if m.sessionIdx < 0 || m.sessionIdx >= len(m.sessionList) {
		return nil
	}
	if m.cfg.ResumeSession == nil {
		m.hint = "宿主未注入会话恢复能力"
		return nil
	}
	id := m.sessionList[m.sessionIdx].ID
	events, err := m.cfg.ResumeSession(id)
	if err != nil {
		m.hint = "恢复失败: " + err.Error()
		return nil
	}
	m.sessionID = id
	m.blocks = nil
	m.term = ""
	m.resetTurnStats()
	m.resetSessionStats()
	m.showSessions = false
	for _, ev := range events {
		m.applyEvent(ev)
	}
	m.hint = "已恢复会话 " + id + "（下一次输入会续接该会话）"
	m.follow = true
	m.refreshView()
	return nil
}

func (m *Model) handleCtrlC() tea.Cmd {
	// 空闲时先关模态层，不直接退出。
	if !m.busy() && m.showHelp {
		m.showHelp = false
		m.hint = ""
		return nil
	}
	if !m.busy() && m.showSessions {
		m.showSessions = false
		m.hint = ""
		return nil
	}
	if m.busy() {
		m.lastCtrlC = time.Now()
		return m.cancel()
	}
	if !m.lastCtrlC.IsZero() && time.Since(m.lastCtrlC) <= 2*time.Second {
		return tea.Quit
	}
	m.lastCtrlC = time.Now()
	m.hint = "再按一次 Ctrl+C 退出"
	return nil
}

// cancel 处理 Ctrl-C / 忙碌时的 Esc：中断当前回合并清空队列。
func (m *Model) cancel() tea.Cmd {
	wasBusy := m.busy()
	if !wasBusy {
		if m.showHelp {
			m.showHelp = false
			return nil
		}
		if m.showSessions {
			m.showSessions = false
			m.hint = ""
			return nil
		}
		return tea.Quit
	}
	// 忙碌时 Ctrl+C 的语义是中断回合，模态层不能吞掉它。
	m.showHelp = false
	m.showSessions = false
	if m.pending != nil {
		m.denyPending("已中断")
	}
	if m.cfg.Cancel != nil {
		m.cfg.Cancel()
	}
	// 使旧回合的事件与关闭消息全部失效；等待中的 waitForEvent 会继续排空 channel。
	// draining 保证旧引擎释放会话锁之前，新任务只会排队而不会抢锁。
	if m.run != nil {
		m.draining++
	}
	m.runID++
	m.run = nil
	m.state = stIdle
	m.queue = nil
	m.hint = "已中断，会话已保存；排队任务已清空"
	m.refreshView()
	return nil
}

func (m *Model) setSize(w, h int) {
	m.width, m.height = w, h
	m.input.SetWidth(maxInt(10, w-4))
	m.resetRenderCache()
	m.ready = true
	m.layout()
	m.refreshView()
}

func (m *Model) resetTurnStats() {
	m.turnToolsOK, m.turnToolsFail = 0, 0
	m.turnGateAllow, m.turnGateDeny = 0, 0
	m.turnEvidence = 0
	m.obsRule, m.obsReason = "", ""
}

func (m *Model) resetSessionStats() {
	m.sessToolsOK, m.sessToolsFail = 0, 0
	m.sessGateAllow, m.sessGateDeny = 0, 0
	m.tokensIn, m.tokensOut, m.promptTokens = 0, 0, 0
	m.ctxSystem, m.ctxHB, m.ctxHistory, m.ctxTools = 0, 0, 0, 0
}

func (m *Model) applyTheme(name string) {
	m.cfg.Theme = name
	m.th = theme.Load(name)
	width := m.view.Width
	if width < 20 {
		width = 80
	}
	m.renderer = render.New(width, m.th.Name)
	m.resetRenderCache()
	m.refreshView()
}

func (m *Model) applySettings(s Settings) {
	if s.Model != "" {
		m.cfg.Model = s.Model
	}
	if s.Protocol != "" {
		m.cfg.Protocol = s.Protocol
	}
	if s.BaseURL != "" {
		m.cfg.BaseURL = s.BaseURL
	}
	if s.Theme != "" {
		m.applyTheme(s.Theme)
		return
	}
	m.refreshView()
}

func (m *Model) resetRenderCache() {
	for i := range m.blocks {
		m.blocks[i].dirty = true
		m.blocks[i].mdKey, m.blocks[i].mdOut = 0, ""
	}
}

func (m *Model) verboseCommand(args []string) tea.Cmd {
	if len(args) == 0 || strings.EqualFold(args[0], "all") {
		m.verboseTools = !m.verboseTools
		m.markAllDirty()
		if m.verboseTools {
			m.hint = "工具输出已全部展开（/verbose all 切换）"
		} else {
			m.hint = "工具输出已全部折叠（/verbose n 选择单个）"
		}
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "last":
		m.toggleLastTool()
		return nil
	case "list":
		m.blocks = append(m.blocks, block{
			kind: blkNotice, content: m.toolListText(), dirty: true,
			meta: map[string]string{"level": "info"},
		})
		m.refreshView()
		return nil
	}
	n := 0
	if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil || n <= 0 {
		m.hint = "用法: /verbose [n|all|list|last]"
		return nil
	}
	if m.verboseTools {
		m.verboseTools = false
		m.markAllDirty()
	}
	if !m.toggleToolAt(n) {
		m.hint = fmt.Sprintf("没有第 %d 个工具输出（共 %d 个）", n, m.toolCount())
	}
	return nil
}

func (m *Model) toolCount() int {
	n := 0
	for i := range m.blocks {
		if m.blocks[i].kind == blkTool {
			n++
		}
	}
	return n
}

func (m *Model) toggleToolAt(n int) bool {
	cur := 0
	for i := range m.blocks {
		if m.blocks[i].kind != blkTool {
			continue
		}
		cur++
		if cur != n {
			continue
		}
		m.blocks[i].collapsed = !m.blocks[i].collapsed
		m.blocks[i].dirty = true
		state := "展开"
		if m.blocks[i].collapsed {
			state = "折叠"
		}
		m.hint = fmt.Sprintf("工具 #%d 已%s（/verbose list 查看全部）", n, state)
		m.refreshView()
		return true
	}
	return false
}

func (m *Model) toolListText() string {
	var b strings.Builder
	b.WriteString("工具输出（/verbose n 展开/折叠，/verbose all 全部）：")
	n := 0
	for i := range m.blocks {
		if m.blocks[i].kind != blkTool {
			continue
		}
		n++
		status := "成功"
		if !m.blocks[i].ok {
			status = "失败"
		}
		args := render.Truncate(strings.ReplaceAll(m.blocks[i].meta["args"], "\n", " "), 50)
		fmt.Fprintf(&b, "\n  #%d %s %s · %s", n, m.blocks[i].meta["tool"], args, status)
	}
	if n == 0 {
		b.WriteString("\n  （无）")
	}
	return b.String()
}

func (m *Model) toggleLastTool() bool {
	n := m.toolCount()
	if n == 0 {
		return false
	}
	return m.toggleToolAt(n)
}

func (m *Model) markAllDirty() {
	for i := range m.blocks {
		m.blocks[i].dirty = true
		m.blocks[i].mdKey, m.blocks[i].mdOut = 0, ""
	}
	m.refreshView()
}

func (m *Model) markToolsDirty() {
	for i := range m.blocks {
		if m.blocks[i].kind == blkTool {
			m.blocks[i].dirty = true
		}
	}
	m.refreshView()
}

// refreshView 只渲染 dirty 块，其余用缓存；跟随底部时自动滚到底。
func (m *Model) refreshView() {
	width := maxInt(20, m.view.Width)
	var sb strings.Builder
	for i := range m.blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(m.blocks[i].RenderWith(m.renderer, width, m.th, m.verboseTools))
	}
	m.view.SetContent(sb.String())
	if m.view.Height < 1 {
		m.view.Height = 1
	}
	if m.follow {
		m.view.GotoBottom()
		m.newBelow = false
	} else if m.view.TotalLineCount() > 0 {
		m.newBelow = true
	}
}

func (m *Model) contextPercent() int {
	window := m.cfg.ContextWindow
	if window <= 0 {
		window = 960000
	}
	used := m.promptTokens
	if used <= 0 {
		return 100
	}
	pct := 100 - used*100/window
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}
