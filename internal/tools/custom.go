package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// CustomTool 是 tools.d/*.json 声明的本地命令工具。
// 命令直接 exec，不经过 shell；参数用 {name} 模板替换，避免命令拼接注入。
type CustomTool struct {
	name    string
	desc    string
	command string
	args    []string
	schema  json.RawMessage
	risk    Risk
	workdir string
	timeout time.Duration
}

type customSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Command     string          `json:"command"`
	Args        []string        `json:"args"`
	Risk        string          `json:"risk"`
	Schema      json.RawMessage `json:"schema"`
	TimeoutMS   int             `json:"timeout_ms"`
}

func (t *CustomTool) Name() string            { return t.name }
func (t *CustomTool) Description() string     { return t.desc }
func (t *CustomTool) Schema() json.RawMessage { return t.schema }
func (t *CustomTool) Risk() Risk              { return t.risk }

func (t *CustomTool) Run(ctx context.Context, raw json.RawMessage) (Result, error) {
	var values map[string]any
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &values); err != nil {
			return failResult("参数不是 JSON object: " + err.Error())
		}
	}
	args := make([]string, 0, len(t.args))
	for _, tmpl := range t.args {
		args = append(args, substTemplate(tmpl, values))
	}
	runCtx := ctx
	if t.timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(runCtx, t.command, args...)
	cmd.Dir = t.workdir
	out, err := cmd.CombinedOutput()
	output := string(out)
	if len(output) > 1<<20 {
		output = output[:1<<20] + "\n… 输出超过 1MB，已截断"
	}
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			return failResult("启动自定义工具失败: " + err.Error())
		}
	}
	return Result{
		Output: output, OK: exit == 0,
		Meta:       map[string]string{"exit_code": strconv.Itoa(exit)},
		Provenance: core.ProvObserved,
	}, nil
}

func substTemplate(tmpl string, values map[string]any) string {
	for key, value := range values {
		tmpl = strings.ReplaceAll(tmpl, "{"+key+"}", fmt.Sprint(value))
	}
	return tmpl
}

// LoadCustomTools 读取目录下所有 *.json 工具声明，按文件名排序。
func LoadCustomTools(dir, workdir string) ([]Tool, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workdir, dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Tool
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		var spec customSpec
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		tool, err := newCustomTool(spec, workdir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func newCustomTool(spec customSpec, workdir string) (*CustomTool, error) {
	if strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Command) == "" {
		return nil, fmt.Errorf("name 与 command 必填")
	}
	risk, ok := parseCustomRisk(spec.Risk)
	if !ok {
		return nil, fmt.Errorf("risk 必须是 read/write/exec/net")
	}
	if len(spec.Schema) == 0 {
		spec.Schema = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	timeout := 30 * time.Second
	if spec.TimeoutMS > 0 {
		timeout = time.Duration(spec.TimeoutMS) * time.Millisecond
	}
	return &CustomTool{
		name: spec.Name, desc: spec.Description, command: spec.Command,
		args: spec.Args, schema: spec.Schema, risk: risk, workdir: workdir, timeout: timeout,
	}, nil
}

func parseCustomRisk(s string) (Risk, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "read":
		return RiskRead, true
	case "write":
		return RiskWrite, true
	case "exec", "":
		return RiskExec, true
	case "net":
		return RiskNet, true
	default:
		return 0, false
	}
}

// RegisterCustomTools 加载并注册目录中的工具；名称冲突直接报错。
func RegisterCustomTools(r *Registry, dir, workdir string) error {
	list, err := LoadCustomTools(dir, workdir)
	if err != nil {
		return err
	}
	for _, tool := range list {
		if err := r.Register(tool); err != nil {
			return err
		}
	}
	return nil
}
