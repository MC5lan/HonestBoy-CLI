package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

type Config struct {
	Workdir         string
	Model           string
	Protocol        string
	BaseURL         string
	APIKey          string
	Client          llm.Client
	Registry        *tools.Registry
	Store           *store.SessionStore
	Approval        ApprovalPolicy
	ApprovalHandler ApprovalHandler // TUI 可注入交互式审批；nil 时按策略自动拒绝
	MaxTurns        int
	AutoWrite       bool
	ContextWindow   int  // 模型上下文窗口，用于预算裁剪
	HBMDLimit       int  // HB.md 读取上限
	RepoMapTokens   int  // 仓库符号地图 token 预算；0 用默认 1200，负数关闭
	PlanMode        bool // 计划模式：只读探索，不产出写操作
	AutoCommit      bool // Aider 风格：回合结束自动 git commit 并产出 diff_review 事件
	SelfAssessment  bool // 自评块（文档 §4.1）：conf=unknown 阻断写操作
	AmbiguityHalt   bool // 歧义暂停（文档 §4.2）
	LedgerRecall    bool // 历史失败案例注入（文档 §3.5）
	PlanFirst       bool // 写操作前必须给测试计划（文档 §4.3）
	// MaxOutputTokens 是单次模型响应的输出预算（含推理 token）。
	// 不能按轮数估算：推理模型会把预算耗在思考上，导致空响应。
	MaxOutputTokens int
	// ReasoningEffort 是思考强度（low/high/max）；空表示用提供方默认。
	ReasoningEffort string
	// Rigor 是质量契约等级：normal | strict。
	Rigor  string
	Tracer *core.Tracer
	// ResumeID 非空时续接已有会话（事件与证据链追加到同一文件）。
	ResumeID string
	// HistoryLimit 是注入上一轮历史的 rune 上限；0 用默认 16000。
	HistoryLimit int
}

// ResolveClient 按配置解析模型客户端；显式注入优先。
func (c Config) ResolveClient() llm.Client {
	if c.Client != nil {
		return c.Client
	}
	protocol := llm.DetectProtocol(c.Protocol, c.BaseURL)
	return llm.NewClient(protocol, c.BaseURL, c.APIKey, c.Model)
}

// Engine 是一次运行的编排器。
type Engine struct {
	cfg             Config
	bus             *EventBus
	gate            *core.Gate
	store           *store.SessionStore
	session         *core.Session
	out             chan<- core.Event
	ledger          *core.Ledger
	guard           *core.LoopGuard
	hook            *EvidenceHook
	usage           llm.Usage
	lastReceipt     string
	registry        *tools.Registry
	approval        ApprovalPolicy
	approvalHandler ApprovalHandler
	approvedAll     map[string]bool
	didWrite        bool
	startTime       time.Time
	ledgerFile      *os.File
}

func New(cfg Config) *Engine {
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 20
	}
	if cfg.Registry == nil {
		cfg.Registry = tools.NewRegistry()
	}
	if cfg.Store == nil {
		cfg.Store = store.NewSessionStore(store.DefaultPaths())
	}
	if cfg.Approval.Write == "" {
		cfg.Approval.Write = "auto"
	}
	if cfg.Approval.Exec == "" {
		cfg.Approval.Exec = "ask"
	}
	if cfg.Approval.Net == "" {
		cfg.Approval.Net = "ask"
	}
	if cfg.MaxOutputTokens <= 0 {
		// 推理模型把思考算进输出预算：max 档给 128k，其余 64k。
		if strings.EqualFold(cfg.ReasoningEffort, "max") {
			cfg.MaxOutputTokens = 128000
		} else {
			cfg.MaxOutputTokens = 65536
		}
	}
	if cfg.Rigor == "" {
		cfg.Rigor = "normal"
	}
	if cfg.ContextWindow > 0 && cfg.MaxOutputTokens > cfg.ContextWindow/2 {
		cfg.MaxOutputTokens = cfg.ContextWindow / 2
	}
	e := &Engine{
		cfg: cfg, bus: NewEventBus(), gate: core.NewGate(core.Policy{Strict: true}),
		store: cfg.Store, registry: cfg.Registry, approval: cfg.Approval,
		approvalHandler: cfg.ApprovalHandler, approvedAll: make(map[string]bool),
		ledger: &core.Ledger{}, guard: &core.LoopGuard{}, hook: NewEvidenceHook(),
	}
	e.openLedger()
	return e
}

// openLedger 加载历史失败账本并打开追加文件。
func (e *Engine) openLedger() {
	dir := e.store.Paths.Root
	_ = os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, "ledger.jsonl")
	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1<<20)
		for scanner.Scan() {
			var entry core.LedgerEntry
			if json.Unmarshal(scanner.Bytes(), &entry) == nil {
				e.ledger.Record(entry)
			}
		}
		f.Close()
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		e.ledgerFile = f
	}
}

// recordFailure 写入内存账本并追加落盘。
func (e *Engine) recordFailure(entry core.LedgerEntry) {
	e.ledger.Record(entry)
	if e.ledgerFile != nil {
		if b, err := json.Marshal(entry); err == nil {
			e.ledgerFile.Write(append(b, '\n'))
		}
	}
}

// Run 启动一个回合并返回事件流。事件是唯一真相，同一份流供 CLI/日志/回放消费。
func (e *Engine) Run(ctx context.Context, prompt string) (<-chan core.Event, <-chan error) {
	events := make(chan core.Event, 64)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		defer recoverCrash(e)
		defer func() {
			if e.ledgerFile != nil {
				_ = e.ledgerFile.Close()
			}
		}()
		e.out = events
		e.startTime = time.Now().UTC()

		var s *core.Session
		if e.cfg.ResumeID != "" {
			loaded, err := e.store.Open(e.cfg.ResumeID)
			if err != nil {
				errs <- err
				return
			}
			s = loaded
		} else {
			s = &core.Session{
				ID:  "s_" + shortID(prompt+time.Now().UTC().Format(time.RFC3339Nano)),
				Cwd: e.cfg.Workdir, CreatedAt: e.startTime,
			}
			if err := e.store.Create(*s); err != nil {
				errs <- err
				return
			}
		}
		e.session = s
		defer func() { _ = e.store.Unlock() }()

		if e.cfg.AutoCommit {
			e.prepareUserChanges()
		}

		historyLimit := e.cfg.HistoryLimit
		if historyLimit <= 0 {
			historyLimit = 16000
		}
		history := historyText(s, historyLimit)
		terminal, err := e.runTurn(ctx, prompt, history)
		if err != nil {
			// 错误必须作为事件发出：TUI 只消费事件流，不读 errs channel。
			e.emit(core.EvtError, core.ErrorData{
				Code: "HB-TOOL-001", Message: err.Error(),
			})
			select {
			case errs <- err:
			default:
			}
		}
		if e.cfg.AutoCommit && e.didWrite {
			e.autoCommit(prompt)
		}
		e.emit(core.EvtTurnEnd, core.TurnEndData{
			Terminal: string(terminal), DurationMS: time.Since(e.startTime).Milliseconds(),
			TokensIn: e.usage.In, TokensOut: e.usage.Out, ReceiptID: e.lastReceipt,
		})
		e.emit(core.EvtSessionEnd, core.SessionEndData{Terminal: string(terminal)})
	}()
	return events, errs
}

// emit 是引擎内唯一产生事件的出口：写会话、写内存、广播、推给消费者。
func (e *Engine) emit(t core.EventType, data any) core.Event {
	ev := e.session.Emit(t, data)
	if e.store != nil {
		_ = e.store.AppendEvent(ev)
	}
	if e.out != nil {
		e.out <- ev
	}
	if payload, err := json.Marshal(ev); err == nil {
		e.bus.Publish(payload)
	}
	return ev
}

// writeReceipt 落一条 Receipt（halt 由中断产生时不写）。
func (e *Engine) writeReceipt(terminal core.Terminal, claim string) string {
	s := e.session
	r := core.NewReceipt(
		"rc_"+shortID(s.ID+claim+terminal.String()),
		s.ID, currentTurnID(s), terminal, claim,
		e.hook.Bag().All(), s.ChainTail(), time.Now().UTC(),
	)
	if err := s.AppendReceipt(r); err != nil {
		return ""
	}
	_ = e.store.AppendReceipt(r)
	e.lastReceipt = r.ID
	return r.ID
}

func currentTurnID(s *core.Session) string {
	if t := s.LastTurn(); t != nil {
		return t.ID
	}
	return ""
}

// toolSpecs 把注册表声明转换为协议无关的工具描述。
func toolSpecs(r *tools.Registry) []llm.ToolSpec {
	list := r.List()
	out := make([]llm.ToolSpec, 0, len(list))
	for _, t := range list {
		out = append(out, llm.ToolSpec{Name: t.Name(), Description: t.Description(), Schema: t.Schema()})
	}
	return out
}

// systemPrompt 只陈述系统如何运转（结构性规则），不做道德劝说。
// 依据设计文档 F1：不写“你必须诚实”类表述——它无效且会强化表演。
// “完成判定”由 Gate 在代码层强制执行，不依赖模型遵守提示。
const systemPrompt = `你是 Honest Boy，一个本地编程智能体。

系统运转规则（描述事实，不是对你的要求）：
1. “完成”状态由独立门禁写入。完成所需证据由本回合的实际改动决定：
   - write_file / edit_file / apply_patch 成功：文件变更证据（diff）
   - shell 成功：命令证据（command）
   - verify_build / verify_test / verify_vet 实际运行：build / test / vet 证据
   .go 文件需要 build + test + diff；.md/.json/.yaml 需要 file + diff + 回读；
   其他文件（如 .html）需要 diff；执行过 shell 需要 command。自然语言不构成状态变更。
2. 想结束回合，须调用 request_done，参数为 {"summary":"本回合摘要"}；门禁会检查证据覆盖全部改动目标，不满足则把缺口作为工具结果回传，最多补救两轮。
3. 无法验证时调用 declare_untested，参数为 {"reason":"...","not_done":["..."],"done":[...],"next":"..."}；reason 与 not_done 必填。这是一个合法的三态出口（done / untested / denied）。
4. 验证器输出原文由系统原样保留并截断；下游改写不会影响已记录的哈希链。
5. 同一失败重复三次，回合自动转入 halt。

工具用法：
- write_file 必须同时提供 path 与 content；read_file 会记录文件哈希，随后才能 edit_file。
- edit_file 的 old 片段必须唯一，否则报错；apply_patch 接受 *** Begin Patch 格式。
- shell 结果会附带退出码；空输出不代表失败，以退出码为准。shell 有超时与输出上限，超时由系统中断。`
