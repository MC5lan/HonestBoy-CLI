package tui

import (
	"errors"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/render"
	"github.com/MC5lan/HonestBoy-CLI/internal/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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

// Config 由 cmd 层装配；tui 不直接依赖 engine/tools，避免依赖方向违规。
type Config struct {
	Workdir       string
	Model         string
	Theme         string
	History       []string
	Run           func(string) (<-chan core.Event, <-chan error)
	Cancel        func()
	Approvals     <-chan ApprovalAsk
	Replay        []core.Event       // 恢复会话时回放的历史事件
	ContextWindow int                // 上下文窗口，用于状态栏百分比
	Protocol      string             // 当前协议（/config 显示）
	BaseURL       string             // 当前端点（/config 显示）
	SetModel      func(string) error // 切换模型并保存配置；nil 时仅本会话生效
}

type Model struct {
	width, height int
	cfg           Config
	state         state
	blocks        []block
	input         textinput.Model
	view          viewport.Model
	spin          spinner.Model
	events        <-chan core.Event
	errCh         <-chan error
	hint          string
	err           error
	term          core.Terminal
	startedAt     time.Time
	lastRender    time.Time
	renderer      *render.Renderer
	th            theme.Theme
	history       []string
	histIdx       int
	ready         bool
	interruptSeen bool
	tokensIn      int
	tokensOut     int
	pending       *ApprovalAsk
	spinnerIdx    int
	kickFrame     int // >0 表示正在射门动画
	pitchTick     int // 球场动画帧计数
	cmdIdx        int // 命令面板选中项
	// 可观测计数（直接由事件累加，不依赖 block 状态）
	obsToolsOK   int
	obsToolsFail int
	obsAllow     int
	obsDeny      int
	obsRule      string
	obsReason    string
}

func NewModel(cfg Config) *Model {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = "输入任务…"
	in.Focus()
	in.CharLimit = 0
	in.Width = 76
	th := theme.Load(cfg.Theme)
	m := &Model{
		cfg: cfg, input: in, view: viewport.New(80, 20),
		spin: spinner.New(spinner.WithSpinner(spinner.Dot)),
		th:   th, renderer: render.New(80, th.Name),
		history: append([]string{}, cfg.History...),
	}
	m.histIdx = len(m.history)
	for _, ev := range cfg.Replay {
		m.applyEvent(ev)
	}
	if n := len(cfg.Replay); n > 0 && cfg.Replay[n-1].Type == core.EvtTurnStart {
		m.hint = "上次会话中断，可继续输入或 /clear 清空"
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick, waitForApproval(m.cfg.Approvals))
}

// Submit 启动一次回合；实际执行由 cmd 层注入的 Run 回调完成。
func (m *Model) submit() tea.Cmd {
	prompt := strings.TrimSpace(m.input.Value())
	if prompt == "" {
		return nil
	}
	if name, _, ok := SlashCommand(prompt); ok {
		return m.runSlash(name, prompt)
	}
	m.input.SetValue("")
	m.history = append(m.history, prompt)
	m.histIdx = len(m.history)
	m.blocks = append(m.blocks, block{kind: blkUser, content: prompt, dirty: true})
	m.state = stStreaming
	m.startedAt = time.Now()
	m.err = nil
	m.hint = ""
	if m.cfg.Run == nil {
		m.err = errNoRunner
		m.state = stIdle
		return nil
	}
	m.events, m.errCh = m.cfg.Run(prompt)
	m.refreshView()
	return tea.Batch(waitForEvent(m.events), spinnerTick())
}

func (m *Model) runSlash(name, line string) tea.Cmd {
	m.input.SetValue("")
	_, args, _ := SlashCommand(line)
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
		m.hint = "已清空当前会话显示"
	case "/help":
		m.hint = "Enter 提交 · Ctrl+C 中断(两次退出) · Ctrl+L 清屏 · Tab 补全命令 · 输入 / 看命令面板"
	case "/setup":
		m.hint = "配置向导：退出 TUI 后运行 hb init"
	case "/verify", "/stats", "/sessions", "/context", "/compact":
		m.hint = name + " 由命令行实现：退出 TUI 后运行 hb " + strings.TrimPrefix(name, "/")
	default:
		m.hint = "未知命令 " + name + "；相近: " + strings.Join(suggestCommands(name), ", ")
	}
	return nil
}

// Cancel 处理 Ctrl-C：第一次取消当前回合，第二次退出。
func (m *Model) cancel() tea.Cmd {
	if m.state == stIdle {
		return tea.Quit
	}
	if m.interruptSeen {
		return tea.Quit
	}
	m.interruptSeen = true
	if m.cfg.Cancel != nil {
		m.cfg.Cancel()
	}
	m.hint = "已中断，会话已保存"
	m.state = stIdle
	return nil
}

func (m *Model) setSize(w, h int) {
	m.width, m.height = w, h
	l := ComputeLayout(w, h)
	m.input.Width = w - 4
	if m.input.Width < 10 {
		m.input.Width = 10
	}
	m.view.Width = w - 6 // 对话框左右边框(2) + padding(2) + 余量
	if m.view.Width < 10 {
		m.view.Width = 10
	}
	m.view.Height = l.ViewHeight
	if m.view.Height < 1 {
		m.view.Height = 1
	}
	m.renderer.Invalidate(w - 4)
	m.ready = true
	m.refreshView()
}
