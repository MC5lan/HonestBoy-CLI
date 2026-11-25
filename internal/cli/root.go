package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// GlobalOptions 是全局 flag 的集合。
type GlobalOptions struct {
	Model      string
	Protocol   string
	BaseURL    string
	Cwd        string
	JSON       bool
	Quiet      bool
	Plan       bool
	AutoCommit bool
	Diff       bool
	Profile    bool
	Yes        bool
	NoColor    bool
	Debug      bool
	Timeout    string
	MaxTurns   int
	// AutoResume 让 `hb run` 在没有新任务时自动续跑最近一条未完成会话。
	AutoResume bool
}

// Execute 解析参数并分派命令。无第三方 CLI 依赖。
func Execute(args []string) error {
	opts := GlobalOptions{MaxTurns: 20}
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Println(VersionString())
			return nil
		case "--help", "-h":
			PrintHelp(os.Stdout)
			return nil
		case "--json":
			opts.JSON = true
		case "--quiet":
			opts.Quiet = true
		case "--plan":
			opts.Plan = true
		case "--auto-commit":
			opts.AutoCommit = true
		case "--diff":
			opts.Diff = true
		case "--profile":
			opts.Profile = true
		case "--yes":
			opts.Yes = true
		case "--no-color":
			opts.NoColor = true
		case "--debug":
			opts.Debug = true
		case "--model":
			if v, ok := takeValue(args, &i); ok {
				opts.Model = v
			} else {
				return ErrUsage
			}
		case "--protocol":
			if v, ok := takeValue(args, &i); ok {
				opts.Protocol = v
			} else {
				return ErrUsage
			}
		case "--base-url":
			if v, ok := takeValue(args, &i); ok {
				opts.BaseURL = v
			} else {
				return ErrUsage
			}
		case "--cwd":
			if v, ok := takeValue(args, &i); ok {
				opts.Cwd = v
			} else {
				return ErrUsage
			}
		case "--timeout":
			if v, ok := takeValue(args, &i); ok {
				opts.Timeout = v
			} else {
				return ErrUsage
			}
		case "--max-turns":
			if v, ok := takeValue(args, &i); ok {
				n, err := strconv.Atoi(v)
				if err != nil || n <= 0 {
					return ErrUsage
				}
				opts.MaxTurns = n
			} else {
				return ErrUsage
			}
		case "--auto-resume":
			opts.AutoResume = true
		default:
			rest = append(rest, args[i])
		}
	}
	if opts.JSON && opts.Quiet {
		return ErrUsage
	}
	if opts.Protocol != "" {
		switch strings.ToLower(opts.Protocol) {
		case "openai-chat", "anthropic", "anthropic-messages", "messages", "openai",
			"responses", "openai-responses", "responses-api":
		default:
			return ErrUsage
		}
	}
	if len(rest) == 0 {
		if needsSetup() {
			fmt.Fprintln(os.Stdout, "看起来是第一次使用 Honest Boy，先做一下设置：")
			return RunInit(opts, nil)
		}
		if hasTTY() {
			return ErrInteractive
		}
		PrintHelp(os.Stdout)
		return nil
	}
	cmd, tail := rest[0], rest[1:]
	switch cmd {
	case "run":
		return RunRun(opts, tail)
	case "chat":
		if !hasTTY() {
			return fmt.Errorf("%w: chat 需要 TTY", ErrEnvironment)
		}
		return ErrInteractive
	case "sessions":
		return RunSessions(opts, tail)
	case "resume":
		// hb resume --last：非交互续跑当前工作区最近一条未完成会话。
		if len(tail) > 0 && (tail[0] == "--last" || tail[0] == "-c") {
			return RunResumeLast(opts, tail[1:])
		}
		if !hasTTY() {
			return fmt.Errorf("%w: resume <id> 需要 TTY；非交互续跑用 hb resume --last", ErrEnvironment)
		}
		if len(tail) < 1 {
			return ErrUsage
		}
		return &InteractiveRequest{ResumeID: tail[0]}
	case "verify":
		return RunVerify(opts, tail)
	case "stats":
		return RunStats(opts, tail)
	case "probe":
		return RunProbe(opts, tail)
	case "config":
		return RunConfig(opts, tail)
	case "doctor":
		return RunDoctor(opts, tail)
	case "init":
		return RunInit(opts, tail)
	case "undo":
		return RunUndo(opts, tail)
	case "diff":
		return RunDiff(opts, tail)
	case "approve":
		return RunApprove(opts, tail)
	case "rewind":
		return RunRewind(opts, tail)
	case "gc":
		return RunGC(opts, tail)
	case "upgrade":
		return RunUpgrade(opts, tail)
	case "export":
		return RunExport(opts, tail)
	case "explain":
		if len(tail) < 1 {
			return ErrUsage
		}
		return RunExplain(opts, tail)
	case "help":
		if len(tail) > 0 {
			if !PrintHelpTopic(os.Stdout, tail[0]) {
				return fmt.Errorf("%w: 未知帮助主题 %s（可用 run/config/evidence/tools/exit-codes）", ErrUsage, tail[0])
			}
			return nil
		}
		PrintHelp(os.Stdout)
		return nil
	default:
		return ErrUsage
	}
}

// needsSetup 判断是否为首次运行（无配置文件或没有端点）。
func needsSetup() bool {
	cfg, err := store.Load()
	if err != nil {
		return true
	}
	return cfg.BaseURL == ""
}

// hasTTY 判断 stdout 是否终端；非 TTY 时 TUI 自动降级为帮助。
func hasTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

// takeValue 消费一个 flag 的值参数。
func takeValue(args []string, i *int) (string, bool) {
	if *i+1 >= len(args) {
		return "", false
	}
	*i++
	return args[*i], true
}

var _ io.Writer = os.Stdout
