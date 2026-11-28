package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/diff"
)

// ApplyPatchTool 解析 Codex 风格的 *** Begin Patch 补丁，支持 Add/Delete/Update File。
// 与 edit_file 一样是精确替换：锚点不唯一就报错，不做模糊匹配。
type ApplyPatchTool struct {
	sb    *Sandbox
	guard *ReadGuard
}

func NewApplyPatchTool(sb *Sandbox, guard *ReadGuard) *ApplyPatchTool {
	return &ApplyPatchTool{sb: sb, guard: guard}
}
func (t *ApplyPatchTool) Name() string { return "apply_patch" }
func (t *ApplyPatchTool) Description() string {
	return "应用 *** Begin Patch 补丁（新增/删除/更新文件）"
}
func (t *ApplyPatchTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"patch":{"type":"string","description":"以 *** Begin Patch 开头、*** End Patch 结尾的补丁"}},"required":["patch"]}`)
}
func (t *ApplyPatchTool) Risk() Risk { return RiskWrite }

type patchSection struct {
	op    string // add / delete / update
	path  string
	lines []string // 去掉前缀后的 hunk 行（含 ' ' '-' '+' 标记由 parser 保留）
}

func parsePatch(patch string) ([]patchSection, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	started, ended := false, false
	var sections []patchSection
	var cur *patchSection
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "*** Begin Patch":
			started = true
		case trimmed == "*** End Patch":
			ended = true
		case strings.HasPrefix(trimmed, "*** Update File:"):
			sections = append(sections, patchSection{op: "update", path: strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Update File:"))})
			cur = &sections[len(sections)-1]
		case strings.HasPrefix(trimmed, "*** Add File:"):
			sections = append(sections, patchSection{op: "add", path: strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Add File:"))})
			cur = &sections[len(sections)-1]
		case strings.HasPrefix(trimmed, "*** Delete File:"):
			sections = append(sections, patchSection{op: "delete", path: strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Delete File:"))})
			cur = &sections[len(sections)-1]
		case strings.HasPrefix(trimmed, "*** Move to:"), trimmed == "@@":
			continue
		case started && cur != nil && trimmed != "":
			cur.lines = append(cur.lines, line)
		}
	}
	if !started || !ended {
		return nil, fmt.Errorf("补丁必须以 *** Begin Patch 开头并以 *** End Patch 结尾")
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("补丁中没有文件操作")
	}
	return sections, nil
}

// applyHunks 把 hunk 行转成 (old,new) 文本并做精确替换。
func applyHunks(content string, lines []string) (string, error) {
	var oldB, newB strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "+") {
			newB.WriteString(strings.TrimPrefix(line, "+"))
			newB.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "-") {
			oldB.WriteString(strings.TrimPrefix(line, "-"))
			oldB.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, " ") {
			text := strings.TrimPrefix(line, " ")
			oldB.WriteString(text)
			oldB.WriteString("\n")
			newB.WriteString(text)
			newB.WriteString("\n")
		}
	}
	oldText, newText := oldB.String(), newB.String()
	if oldText == "" {
		return "", fmt.Errorf("补丁 hunk 没有上下文或删除行")
	}
	count := strings.Count(content, oldText)
	if count == 0 {
		return "", fmt.Errorf("未找到匹配的补丁锚点")
	}
	if count > 1 {
		return "", fmt.Errorf("补丁锚点匹配到 %d 处，请提供更多上下文", count)
	}
	return strings.Replace(content, oldText, newText, 1), nil
}

func (t *ApplyPatchTool) Run(ctx context.Context, args json.RawMessage) (Result, error) {
	var a struct {
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal(args, &a); err != nil || strings.TrimSpace(a.Patch) == "" {
		return failResult("参数错误: patch 必填")
	}
	sections, err := parsePatch(a.Patch)
	if err != nil {
		return failResult("补丁解析失败: " + err.Error())
	}
	var changed []string
	var diffs []string
	for _, sec := range sections {
		if sec.path == "" {
			return failResult("补丁中的文件路径为空")
		}
		switch sec.op {
		case "add":
			var b strings.Builder
			for _, line := range sec.lines {
				if strings.HasPrefix(line, "+") {
					b.WriteString(strings.TrimPrefix(line, "+"))
					b.WriteString("\n")
				}
			}
			if err := t.writeAtomic(sec.path, []byte(b.String()), nil); err != nil {
				return failResult("新增文件失败: " + err.Error())
			}
			diffs = append(diffs, diff.Unified(sec.path, "", b.String()))
			changed = append(changed, sec.path+" (新增)")
		case "delete":
			existing, err := t.sb.ReadFile(sec.path)
			if err != nil {
				return failResult("删除失败: " + err.Error())
			}
			if err := t.guard.Check(sec.path, existing); err != nil {
				return failResult(err.Error())
			}
			if err := t.sb.Remove(sec.path); err != nil {
				return failResult("删除失败: " + err.Error())
			}
			diffs = append(diffs, diff.Unified(sec.path, normalizeLF(string(existing)), ""))
			t.guard.MarkRead(sec.path, nil)
			changed = append(changed, sec.path+" (删除)")
		case "update":
			existing, err := t.sb.ReadFile(sec.path)
			if err != nil {
				return failResult("读取失败: " + err.Error())
			}
			if err := t.guard.Check(sec.path, existing); err != nil {
				return failResult(err.Error())
			}
			updated, err := applyHunks(string(existing), sec.lines)
			if err != nil {
				return failResult(sec.path + ": " + err.Error())
			}
			if err := t.writeAtomic(sec.path, []byte(updated), existing); err != nil {
				return failResult("写入失败: " + err.Error())
			}
			diffs = append(diffs, diff.Unified(sec.path, normalizeLF(string(existing)), normalizeLF(updated)))
			changed = append(changed, sec.path+" (更新)")
		}
	}
	// 全部成功后才返回；逐文件原子写，失败的文件已回滚由下一次读后写校验发现。
	return Result{
		Output: "已应用补丁: " + strings.Join(changed, ", "), OK: true,
		Meta: map[string]string{
			"sha256": sha256Hex([]byte(strings.Join(changed, "|"))), "files": fmt.Sprint(len(changed)),
			"diff": strings.Join(diffs, "\n"),
		},
		Provenance: core.ProvObserved,
	}, nil
}

func (t *ApplyPatchTool) writeAtomic(path string, data, existing []byte) error {
	if existing == nil {
		if cur, err := t.sb.ReadFile(path); err == nil {
			if err := t.guard.Check(path, cur); err != nil {
				return err
			}
			existing = cur
		}
	}
	style := "\n"
	if strings.Contains(string(existing), "\r\n") {
		style = "\r\n"
	}
	content := string(data)
	if style == "\r\n" {
		content = strings.ReplaceAll(content, "\r\n", "\n")
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	out := []byte(content)
	tmp := path + ".tmp-" + hex.EncodeToString([]byte(fmt.Sprint(time.Now().UnixNano())))[:8]
	f, err := t.sb.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = f.Write(out); err != nil {
		_ = f.Close()
		_ = t.sb.Remove(tmp)
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		_ = t.sb.Remove(tmp)
		return err
	}
	if err = f.Close(); err != nil {
		_ = t.sb.Remove(tmp)
		return err
	}
	if err = t.sb.Rename(tmp, path); err != nil {
		_ = t.sb.Remove(tmp)
		return err
	}
	_ = sha256.Sum256(out)
	t.guard.MarkRead(path, out)
	return nil
}
