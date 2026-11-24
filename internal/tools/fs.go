package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/diff"
)

const (
	maxReadBytes      = 100 << 20 // 100MB，超过直接拒绝
	maxSingleLineSize = 1 << 20   // 单行 1MB，超过拒绝（TD05 E1：10MB 单行文件）
	maxLineChars      = 2000
	defaultReadLines  = 2000
	maxListEntries    = 500
	maxListDepth      = 4
)

var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "bin": true,
	"dist": true, ".idea": true, "__pycache__": true,
}

func failResult(msg string) (Result, error) {
	return Result{OK: false, Output: msg, Provenance: core.ProvObserved}, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ---------------- read_file ----------------

type ReadFileTool struct {
	sb    *Sandbox
	guard *ReadGuard
}

func NewReadFileTool(sb *Sandbox, guard *ReadGuard) *ReadFileTool {
	return &ReadFileTool{sb: sb, guard: guard}
}
func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string {
	return "读取工作区内的文本文件（带行号）"
}
func (t *ReadFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":1,"maximum":2000}},"required":["path"]}`)
}
func (t *ReadFileTool) Risk() Risk { return RiskRead }

type readArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

func parseReadArgs(raw json.RawMessage) (readArgs, error) {
	var a readArgs
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return a, fmt.Errorf("missing arguments")
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return a, err
	}
	if a.Path == "" {
		return a, fmt.Errorf("path is required")
	}
	if a.Limit <= 0 {
		a.Limit = defaultReadLines
	}
	if a.Limit > defaultReadLines {
		a.Limit = defaultReadLines
	}
	if a.Offset < 0 {
		a.Offset = 0
	}
	return a, nil
}

func (t *ReadFileTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	a, err := parseReadArgs(args)
	if err != nil {
		return failResult("参数错误: " + err.Error())
	}
	info, err := t.sb.Stat(a.Path)
	if err != nil {
		return failResult("读取失败: " + err.Error())
	}
	if info.IsDir() {
		return failResult("这是一个目录，请用 list_dir")
	}
	if info.Size() > maxReadBytes {
		return failResult(ErrFileTooLarge.Error())
	}
	data, err := t.sb.ReadFile(a.Path)
	if err != nil {
		return failResult("读取失败: " + err.Error())
	}
	if bytes.Contains(data[:min(len(data), 8192)], []byte{0}) {
		return failResult(ErrBinaryFile.Error())
	}
	lines := strings.Split(string(data), "\n")
	var out strings.Builder
	end := a.Offset + a.Limit
	if end > len(lines) {
		end = len(lines)
	}
	for i := a.Offset; i < end; i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		if len(line) > maxSingleLineSize {
			// 单行超过 1MB 直接拒绝，不能默默截断成一行——TD05 要求"拒绝并提示"。
			return failResult(ErrLineTooLong.Error() + ": 单行超过 1MB，已拒绝读取；" +
				"请先用 shell 把它切分成正常换行的文件，或有针对性地处理该行。")
		}
		if len([]rune(line)) > maxLineChars {
			line = string([]rune(line)[:maxLineChars]) + " [line truncated]"
		}
		fmt.Fprintf(&out, "%5d│%s\n", i+1, line)
	}
	// 记录读后写校验所需的哈希（T307）：没有这一步，先读再改会被误判为盲改。
	if t.guard != nil {
		t.guard.MarkRead(a.Path, data)
	}
	return Result{
		Output:     out.String(),
		OK:         true,
		Meta:       map[string]string{"sha256": sha256Hex(data), "lines": fmt.Sprint(len(lines))},
		Provenance: core.ProvObserved,
	}, nil
}

// ---------------- write_file ----------------

type WriteFileTool struct {
	sb    *Sandbox
	guard *ReadGuard
}

func NewWriteFileTool(sb *Sandbox, guard *ReadGuard) *WriteFileTool {
	return &WriteFileTool{sb: sb, guard: guard}
}
func (t *WriteFileTool) Name() string { return "write_file" }
func (t *WriteFileTool) Description() string {
	return "覆盖写入工作区内的文件（原子写）"
}
func (t *WriteFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}
func (t *WriteFileTool) Risk() Risk { return RiskWrite }

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a writeArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Path == "" {
		return failResult("参数错误: path 与 content 必填")
	}
	existing, statErr := t.sb.ReadFile(a.Path)
	style := "\n"
	if statErr == nil {
		if err := t.guard.Check(a.Path, existing); err != nil {
			return failResult(err.Error())
		}
		if bytes.Contains(existing, []byte("\r\n")) {
			style = "\r\n"
		}
	} else if strings.HasSuffix(strings.ToLower(a.Path), ".bat") || strings.HasSuffix(strings.ToLower(a.Path), ".ps1") {
		style = "\r\n"
	}
	content := a.Content
	if style == "\r\n" {
		content = strings.ReplaceAll(content, "\r\n", "\n")
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	tmp := a.Path + ".tmp-" + hex.EncodeToString([]byte(fmt.Sprint(time.Now().UnixNano())))[:8]
	// 错误码放在 OS 原文前：CLI 只显示前 70 字符，可操作建议不能被截掉。
	const ioAdvice = "（HB-IO-001：检查路径权限、只读属性与剩余空间）"
	f, err := t.sb.Create(tmp)
	if err != nil {
		return failResult("创建临时文件失败" + ioAdvice + ": " + err.Error())
	}
	if _, err = f.WriteString(content); err != nil {
		f.Close()
		t.sb.Remove(tmp)
		return failResult("写入失败" + ioAdvice + ": " + err.Error())
	}
	if err = f.Sync(); err != nil {
		f.Close()
		t.sb.Remove(tmp)
		return failResult("同步失败" + ioAdvice + ": " + err.Error())
	}
	if err = f.Close(); err != nil {
		t.sb.Remove(tmp)
		return failResult("关闭失败" + ioAdvice + ": " + err.Error())
	}
	if err = t.sb.Rename(tmp, a.Path); err != nil {
		t.sb.Remove(tmp)
		return failResult("原子替换失败" + ioAdvice + ": " + err.Error())
	}
	t.guard.MarkRead(a.Path, []byte(content))
	before := ""
	if statErr == nil {
		before = normalizeLF(string(existing))
	}
	return Result{
		Output: fmt.Sprintf("已写入 %s (%d 字节)", a.Path, len(content)),
		OK:     true,
		Meta: map[string]string{
			"sha256": sha256Hex([]byte(content)),
			"diff":   diff.Unified(a.Path, before, normalizeLF(content)),
		},
		Provenance: core.ProvObserved,
	}, nil
}

// ---------------- list_dir ----------------

type ListDirTool struct{ sb *Sandbox }

func NewListDirTool(sb *Sandbox) *ListDirTool { return &ListDirTool{sb: sb} }
func (t *ListDirTool) Name() string           { return "list_dir" }
func (t *ListDirTool) Description() string {
	return "列出目录内容（非递归，可配置深度）"
}
func (t *ListDirTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"recursive":{"type":"boolean"},"max_depth":{"type":"integer"}}}`)
}
func (t *ListDirTool) Risk() Risk { return RiskRead }

type listArgs struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	MaxDepth  int    `json:"max_depth"`
}

func (t *ListDirTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a listArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &a); err != nil {
			return failResult("参数错误: " + err.Error())
		}
	}
	if a.Path == "" {
		a.Path = "."
	}
	if a.MaxDepth <= 0 || a.MaxDepth > maxListDepth {
		a.MaxDepth = maxListDepth
	}
	if !a.Recursive {
		a.MaxDepth = 1
	}
	// 路径不存在/越界/不是目录必须给可执行提示；旧实现直接静默返回空列表。
	rel, err := t.sb.Rel(a.Path)
	if err != nil {
		return failResult("路径越界: " + err.Error() + "。请使用工作区内的相对路径。")
	}
	info, err := os.Stat(filepath.Join(t.sb.Workdir(), rel))
	if err != nil {
		return failResult("目录不存在或无法访问: " + a.Path + "。先用 list_dir . 查看当前目录。")
	}
	if !info.IsDir() {
		return failResult("这不是目录，请用 read_file 读取: " + a.Path)
	}
	var lines []string
	truncated := false
	t.listDir(a.Path, "", 1, a.MaxDepth, &lines, &truncated)
	if truncated {
		lines = append(lines, "… 条目超过 500，请用 glob 缩小范围")
	}
	return Result{Output: strings.Join(lines, "\n"), OK: true, Provenance: core.ProvObserved}, nil
}

func (t *ListDirTool) listDir(path, prefix string, depth, maxDepth int, lines *[]string, truncated *bool) {
	if depth > maxDepth || *truncated {
		return
	}
	rel, err := t.sb.Rel(path)
	if err != nil {
		return
	}
	entries, err := os.ReadDir(filepath.Join(t.sb.Workdir(), rel))
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if ignoredDirs[e.Name()] {
			continue
		}
		if len(*lines) >= maxListEntries {
			*truncated = true
			return
		}
		kind := " "
		if e.IsDir() {
			kind = "d"
		}
		size, mod := int64(0), time.Time{}
		if info, infoErr := e.Info(); infoErr == nil {
			size, mod = info.Size(), info.ModTime()
		}
		*lines = append(*lines, fmt.Sprintf("%s %-40s %10d  %s", kind, prefix+e.Name(), size,
			mod.Format("2006-01-02 15:04")))
		if e.IsDir() {
			t.listDir(filepath.Join(path, e.Name()), prefix+e.Name()+string(filepath.Separator), depth+1, maxDepth, lines, truncated)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
