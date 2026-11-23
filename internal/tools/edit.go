package tools

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// EditFileTool 做精确字符串替换，不做模糊匹配。
type EditFileTool struct {
	sb    *Sandbox
	guard *ReadGuard
}

func NewEditFileTool(sb *Sandbox, guard *ReadGuard) *EditFileTool {
	return &EditFileTool{sb: sb, guard: guard}
}
func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "对文件做精确字符串替换" }
func (t *EditFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"},"replace_all":{"type":"boolean"}},"required":["path","old","new"]}`)
}
func (t *EditFileTool) Risk() Risk { return RiskWrite }

type editArgs struct {
	Path       string `json:"path"`
	Old        string `json:"old"`
	New        string `json:"new"`
	ReplaceAll bool   `json:"replace_all"`
}

func normalizeLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func (t *EditFileTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a editArgs
	if err := json.Unmarshal(args, &a); err != nil || a.Path == "" || a.Old == "" {
		return failResult("参数错误: path 与 old 必填")
	}
	existing, err := t.sb.ReadFile(a.Path)
	if err != nil {
		return failResult("读取失败: " + err.Error())
	}
	if err := t.guard.Check(a.Path, existing); err != nil {
		return failResult(err.Error())
	}
	style := "\n"
	if bytes.Contains(existing, []byte("\r\n")) {
		style = "\r\n"
	}
	content := normalizeLF(string(existing))
	old := normalizeLF(a.Old)
	newText := normalizeLF(a.New)
	count := strings.Count(content, old)
	if count == 0 {
		return failResult("未找到匹配")
	}
	if count > 1 && !a.ReplaceAll {
		return failResult(fmt.Sprintf("匹配到 %d 处，请提供更长的上下文", count))
	}
	n := 1
	if a.ReplaceAll {
		n = count
	}
	updated := strings.Replace(content, old, newText, n)
	if style == "\r\n" {
		updated = strings.ReplaceAll(updated, "\n", "\r\n")
	}
	out := []byte(updated)
	tmp := a.Path + ".tmp-" + hex.EncodeToString([]byte(fmt.Sprint(time.Now().UnixNano())))[:8]
	f, err := t.sb.Create(tmp)
	if err != nil {
		return failResult("创建临时文件失败: " + err.Error())
	}
	if _, err = f.Write(out); err != nil {
		f.Close()
		t.sb.Remove(tmp)
		return failResult("写入失败: " + err.Error())
	}
	if err = f.Sync(); err != nil {
		f.Close()
		t.sb.Remove(tmp)
		return failResult("同步失败: " + err.Error())
	}
	if err = f.Close(); err != nil {
		t.sb.Remove(tmp)
		return failResult("关闭失败: " + err.Error())
	}
	if err = t.sb.Rename(tmp, a.Path); err != nil {
		t.sb.Remove(tmp)
		return failResult("原子替换失败: " + err.Error())
	}
	t.guard.MarkRead(a.Path, out)
	return Result{
		Output:     fmt.Sprintf("已编辑 %s（替换 %d 处）", a.Path, n),
		OK:         true,
		Meta:       map[string]string{"sha256": sha256Hex(out), "replacements": fmt.Sprint(n)},
		Provenance: core.ProvObserved,
	}, nil
}
