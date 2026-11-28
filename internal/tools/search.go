package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

type SearchTool struct{ sb *Sandbox }

func NewSearchTool(sb *Sandbox) *SearchTool { return &SearchTool{sb: sb} }
func (t *SearchTool) Name() string          { return "search" }
func (t *SearchTool) Description() string   { return "在工作区内搜索内容（rg 优先）" }
func (t *SearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"glob":{"type":"string"},"max_results":{"type":"integer"}},"required":["pattern"]}`)
}
func (t *SearchTool) Risk() Risk { return RiskRead }

type searchArgs struct {
	Pattern    string `json:"pattern"`
	Glob       string `json:"glob"`
	MaxResults int    `json:"max_results"`
}

type searchHit struct {
	File string
	Line int
	Text string
}

func (t *SearchTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a searchArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Pattern == "" {
		return failResult("参数错误: pattern 必填")
	}
	if _, err := regexp.Compile(a.Pattern); err != nil {
		return failResult("非法正则: " + err.Error())
	}
	if a.MaxResults <= 0 || a.MaxResults > 1000 {
		a.MaxResults = 200
	}
	var hits []searchHit
	if _, err := exec.LookPath("rg"); err == nil {
		var ok bool
		hits, ok = t.rgSearch(ctx, a)
		if !ok {
			hits = t.walkSearch(ctx, a)
		}
	} else {
		hits = t.walkSearch(ctx, a)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
	var out strings.Builder
	lastFile := ""
	for _, h := range hits {
		if h.File != lastFile {
			fmt.Fprintf(&out, "\n%s:\n", h.File)
			lastFile = h.File
		}
		fmt.Fprintf(&out, "  %5d│ %s\n", h.Line, h.Text)
	}
	// 检索结果是线索，不是事实。
	return Result{Output: out.String(), OK: true, Meta: map[string]string{"hits": fmt.Sprint(len(hits))},
		Provenance: core.ProvInferred}, nil
}

// rgSearch 返回 (hits, ok)。ok=false 表示 rg 本身没跑起来或出错，
// 调用方应回退到 Go 遍历；ok=true 时 0 条命中是合法结果，不能再扫一遍。
func (t *SearchTool) rgSearch(ctx context.Context, a searchArgs) ([]searchHit, bool) {
	params := []string{"--json"}
	if a.Glob != "" {
		params = append(params, "-g", a.Glob)
	}
	params = append(params, a.Pattern)
	cmd := exec.CommandContext(ctx, "rg", params...)
	cmd.Dir = t.sb.Workdir()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false
	}
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	var hits []searchHit
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Path struct {
					Text string `json:"text"`
				} `json:"path"`
				LineNumber int `json:"line_number"`
				Lines      struct {
					Text string `json:"text"`
				} `json:"lines"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &ev) != nil || ev.Type != "match" {
			continue
		}
		hits = append(hits, searchHit{
			File: filepath.ToSlash(ev.Data.Path.Text),
			Line: ev.Data.LineNumber,
			Text: strings.TrimRight(ev.Data.Lines.Text, "\r\n"),
		})
		if len(hits) >= a.MaxResults {
			// 常见词可能命中几十万行；拿够结果就停，不等 rg 扫完。
			_ = cmd.Process.Kill()
			break
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if scanErr != nil && len(hits) == 0 {
		return nil, false
	}
	if waitErr != nil && len(hits) == 0 {
		// rg 用 1 表示“没有匹配”，这不是失败。
		var ee *exec.ExitError
		if !errors.As(waitErr, &ee) || ee.ExitCode() != 1 {
			return nil, false
		}
	}
	return hits, true
}

func (t *SearchTool) walkSearch(ctx context.Context, a searchArgs) []searchHit {
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return nil
	}
	var globRe *regexp.Regexp
	if a.Glob != "" {
		pattern := regexp.QuoteMeta(a.Glob)
		pattern = strings.ReplaceAll(pattern, `\*`, `.*`)
		pattern = strings.ReplaceAll(pattern, `\?`, `.`)
		globRe, _ = regexp.Compile("^" + pattern + "$")
	}
	var hits []searchHit
	_ = filepath.WalkDir(t.sb.Workdir(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return nil
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(t.sb.Workdir(), path)
		if relErr != nil {
			return nil
		}
		if globRe != nil && !globRe.MatchString(filepath.Base(rel)) {
			return nil
		}
		data, readErr := readSmallFile(path)
		if readErr != nil || bytes.Contains(data[:min(len(data), 8192)], []byte{0}) {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				hits = append(hits, searchHit{File: filepath.ToSlash(rel), Line: i + 1, Text: strings.TrimSuffix(line, "\r")})
				if len(hits) >= a.MaxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return hits
}

func readSmallFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || info.Size() > maxReadBytes {
		return nil, ErrFileTooLarge
	}
	return io.ReadAll(f)
}
