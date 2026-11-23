package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

const (
	defaultShellTimeout = 120 * time.Second
	maxShellOutput      = 1 << 20 // 1MB
	headLines           = 200
	tailLines           = 100
)

// 环境变量白名单：不继承 HB_API_KEY。
var allowedEnv = []string{
	"PATH", "Path", "HOME", "USERPROFILE", "TEMP", "TMP", "TMPDIR",
	"SystemRoot", "LOCALAPPDATA", "ComSpec", "PATHEXT", "LANG", "LC_ALL",
	"GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "GOPROXY", "CGO_ENABLED",
}

func filteredEnv() []string {
	var out []string
	for _, k := range allowedEnv {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

type ShellTool struct {
	sb    *Sandbox
	shell string
}

func NewShellTool(sb *Sandbox, shell string) *ShellTool { return &ShellTool{sb: sb, shell: shell} }
func (t *ShellTool) Name() string                       { return "shell" }
func (t *ShellTool) Description() string                { return "在沙箱工作目录执行命令" }
func (t *ShellTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"},"timeout_ms":{"type":"integer","minimum":1000,"maximum":600000,"default":120000},"cwd":{"type":"string"}},"required":["command"]}`)
}
func (t *ShellTool) Risk() Risk { return RiskExec }

type shellArgs struct {
	Command   string `json:"command"`
	TimeoutMS int    `json:"timeout_ms"`
	Cwd       string `json:"cwd"`
}

func (t *ShellTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a shellArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Command == "" {
		return failResult("参数错误: command 必填")
	}
	timeout := defaultShellTimeout
	if a.TimeoutMS >= 1000 && a.TimeoutMS <= 600000 {
		timeout = time.Duration(a.TimeoutMS) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if t.shell == "pwsh" {
			cmd = exec.CommandContext(runCtx, "pwsh", "-NoProfile", "-Command", a.Command)
		} else {
			cmd = exec.CommandContext(runCtx, "cmd.exe", "/c", a.Command)
		}
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", a.Command)
	}
	cmd.Env = filteredEnv()
	cmd.Dir = t.sb.Workdir()
	if a.Cwd != "" {
		rel, err := t.sb.Rel(a.Cwd)
		if err != nil {
			return failResult("cwd 越界: " + err.Error())
		}
		cmd.Dir = filepath.Join(t.sb.Workdir(), rel)
	}
	setProcessGroup(cmd)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	start := time.Now()
	err := cmd.Start()
	if err != nil {
		return failResult("启动命令失败: " + err.Error())
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-runCtx.Done():
		killProcessTree(cmd)
		// 子进程可能仍持有 stdout 管道，不能无限等 cmd.Wait。
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		return Result{OK: false, Output: ErrCommandTimeout.Error(), Provenance: core.ProvObserved}, nil
	case waitErr := <-done:
		exitCode := 0
		if waitErr != nil {
			if ee, ok := waitErr.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				return failResult("执行失败: " + waitErr.Error())
			}
		}
		output, truncated := truncateOutput(buf.Bytes())
		return Result{
			Output: output, OK: exitCode == 0,
			Meta: map[string]string{
				"exit_code": fmt.Sprint(exitCode), "duration_ms": fmt.Sprint(time.Since(start).Milliseconds()),
				"truncated": fmt.Sprint(truncated),
			},
			Provenance: core.ProvObserved,
		}, nil
	}
}

// truncateOutput 超限时保留头 200 行 + 尾 100 行，中间标记省略字节数。
func truncateOutput(data []byte) (string, bool) {
	if len(data) <= maxShellOutput {
		return SanitizeOutput(string(data)), false
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) <= headLines+tailLines {
		return SanitizeOutput(string(data[:maxShellOutput])), true
	}
	omitted := len(data) - len([]byte(strings.Join(lines[:headLines], "\n"))) - len([]byte(strings.Join(lines[len(lines)-tailLines:], "\n")))
	head := strings.Join(lines[:headLines], "\n")
	tail := strings.Join(lines[len(lines)-tailLines:], "\n")
	return SanitizeOutput(head + fmt.Sprintf("\n[... 省略 %d 字节 ...]\n", omitted) + tail), true
}

// SanitizeOutput 保证输出是合法 UTF-8。
func SanitizeOutput(s string) string {
	if !utf8.ValidString(s) {
		return string([]rune(s))
	}
	return s
}
