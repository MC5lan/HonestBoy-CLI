package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustArgs(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReadThenEdit(t *testing.T) {
	sb, dir := newTempSandbox(t)
	guard := NewReadGuard()
	write := NewWriteFileTool(sb, guard)
	read := NewReadFileTool(sb, guard)
	edit := NewEditFileTool(sb, guard)
	ctx := context.Background()

	r, err := write.Run(ctx, mustArgs(t, map[string]any{"path": "a.go", "content": "line1\nline2\nline3\n"}))
	if err != nil || !r.OK {
		t.Fatalf("write: %+v %v", r, err)
	}
	r, err = read.Run(ctx, mustArgs(t, map[string]any{"path": "a.go"}))
	if err != nil || !r.OK || !strings.Contains(r.Output, "    1│line1") {
		t.Fatalf("read: %+v %v", r, err)
	}
	if r.Meta["sha256"] == "" {
		t.Fatal("read should report sha256")
	}
	// 先读再改：ReadGuard 应已记录哈希，编辑必须通过。
	r, err = edit.Run(ctx, mustArgs(t, map[string]any{"path": "a.go", "old": "line2", "new": "LINE2"}))
	if err != nil || !r.OK {
		t.Fatalf("edit after read: %+v %v", r, err)
	}
	r, err = read.Run(ctx, mustArgs(t, map[string]any{"path": "a.go"}))
	if err != nil || !strings.Contains(r.Output, "LINE2") {
		t.Fatalf("edited content missing: %+v", r)
	}
	// 外部改动后写入必须拒绝。
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err = write.Run(ctx, mustArgs(t, map[string]any{"path": "a.go", "content": "hijack"}))
	if err != nil || r.OK || !strings.Contains(r.Output, "please read") {
		t.Fatalf("external modification not detected: %+v", r)
	}
}

func TestEditAmbiguousAndNoMatch(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	ctx := context.Background()
	_, _ = (NewWriteFileTool(sb, guard)).Run(ctx, mustArgs(t, map[string]any{
		"path": "b.txt", "content": "x\nx\nx\n",
	}))
	edit := NewEditFileTool(sb, guard)
	r, _ := edit.Run(ctx, mustArgs(t, map[string]any{"path": "b.txt", "old": "x", "new": "y"}))
	if r.OK || !strings.Contains(r.Output, "匹配到 3 处") {
		t.Fatalf("ambiguous: %+v", r)
	}
	r, _ = edit.Run(ctx, mustArgs(t, map[string]any{"path": "b.txt", "old": "zz", "new": "y"}))
	if r.OK || !strings.Contains(r.Output, "未找到匹配") {
		t.Fatalf("no match: %+v", r)
	}
	r, _ = edit.Run(ctx, mustArgs(t, map[string]any{"path": "b.txt", "old": "x", "new": "y", "replace_all": true}))
	if !r.OK || r.Meta["replacements"] != "3" {
		t.Fatalf("replace_all: %+v", r)
	}
}

func TestReadBinaryAndPagination(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(sb.Workdir(), "bin.dat"), []byte{0, 1, 2}, 0o600); err != nil {
		t.Fatal(err)
	}
	r, _ := (NewReadFileTool(sb, guard)).Run(ctx, mustArgs(t, map[string]any{"path": "bin.dat"}))
	if r.OK || !strings.Contains(r.Output, "binary") {
		t.Fatalf("binary not rejected: %+v", r)
	}
	_, _ = (NewWriteFileTool(sb, guard)).Run(ctx, mustArgs(t, map[string]any{
		"path": "many.txt", "content": strings.Repeat("x\n", 10),
	}))
	r, _ = (NewReadFileTool(sb, guard)).Run(ctx, mustArgs(t, map[string]any{"path": "many.txt", "offset": 2, "limit": 3}))
	if !r.OK || strings.Contains(r.Output, "    1│") || !strings.Contains(r.Output, "    3│") {
		t.Fatalf("pagination failed: %+v", r)
	}
}

func TestListDir(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	ctx := context.Background()
	if w, err := (NewWriteFileTool(sb, guard)).Run(ctx, mustArgs(t, map[string]any{
		"path": "c.txt", "content": "c",
	})); err != nil || !w.OK {
		t.Fatalf("write: %+v %v", w, err)
	}
	r, err := (NewListDirTool(sb)).Run(ctx, mustArgs(t, map[string]any{"path": "."}))
	if err != nil || !r.OK || !strings.Contains(r.Output, "c.txt") {
		t.Fatalf("list: %+v %v", r, err)
	}
}

// TD05 E1：10MB 单行必须"拒绝并提示"，不能默默截断。
func TestReadLineTooLong(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	ctx := context.Background()
	huge := strings.Repeat("x", maxSingleLineSize+1)
	if err := os.WriteFile(filepath.Join(sb.Workdir(), "one-line.txt"), []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewReadFileTool(sb, guard).Run(ctx, mustArgs(t, map[string]any{"path": "one-line.txt"}))
	if err != nil || r.OK || !strings.Contains(r.Output, "单行超过 1MB") {
		t.Fatalf("line too long should be rejected: OK=%v out=%q err=%v", r.OK, r.Output, err)
	}
	// 普通长行仍然只截断并继续。
	normal := strings.Repeat("y", maxLineChars+10) + "\nshort\n"
	if err := os.WriteFile(filepath.Join(sb.Workdir(), "normal.txt"), []byte(normal), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err = NewReadFileTool(sb, guard).Run(ctx, mustArgs(t, map[string]any{"path": "normal.txt"}))
	if err != nil || !r.OK || !strings.Contains(r.Output, "[line truncated]") {
		t.Fatalf("normal long line should be truncated: OK=%v out=%q err=%v", r.OK, r.Output, err)
	}
}

// TD04 F05：超过 100MB 的文件在读取前就被拒绝（稀疏文件，不占实际磁盘）。
func TestReadFileTooLarge(t *testing.T) {
	sb, _ := newTempSandbox(t)
	ctx := context.Background()
	f, err := os.Create(filepath.Join(sb.Workdir(), "huge.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxReadBytes + 1); err != nil {
		f.Close()
		t.Skipf("cannot create sparse file: %v", err)
	}
	_ = f.Close()
	r, err := NewReadFileTool(sb, NewReadGuard()).Run(ctx, mustArgs(t, map[string]any{"path": "huge.bin"}))
	if err != nil || r.OK || !strings.Contains(r.Output, "too large") {
		t.Fatalf("huge file should be rejected: OK=%v out=%q err=%v", r.OK, r.Output, err)
	}
}

// TD04 F12：list_dir 路径不存在必须报错而不是静默返回空列表。
func TestListDirMissingPath(t *testing.T) {
	sb, _ := newTempSandbox(t)
	ctx := context.Background()
	r, err := NewListDirTool(sb).Run(ctx, mustArgs(t, map[string]any{"path": "no-such-dir"}))
	if err != nil || r.OK || !strings.Contains(r.Output, "目录不存在") {
		t.Fatalf("missing dir should fail: OK=%v out=%q err=%v", r.OK, r.Output, err)
	}
	r, err = NewListDirTool(sb).Run(ctx, mustArgs(t, map[string]any{"path": "../outside"}))
	if err != nil || r.OK || !strings.Contains(r.Output, "路径越界") {
		t.Fatalf("escape should fail: OK=%v out=%q err=%v", r.OK, r.Output, err)
	}
}

// TD05 E2：list_dir 超过 500 条时截断并提示缩小范围。
func TestListDirTruncatesAt500(t *testing.T) {
	sb, _ := newTempSandbox(t)
	ctx := context.Background()
	for i := 0; i < maxListEntries+5; i++ {
		name := filepath.Join(sb.Workdir(), fmt.Sprintf("f%04d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := NewListDirTool(sb).Run(ctx, mustArgs(t, map[string]any{"path": "."}))
	if err != nil || !r.OK {
		t.Fatalf("list: %+v %v", r, err)
	}
	if !strings.Contains(r.Output, "条目超过 500，请用 glob 缩小范围") {
		t.Fatalf("missing truncation hint: %q", r.Output)
	}
	if got := strings.Count(r.Output, "\n") + 1; got > maxListEntries+1 {
		t.Fatalf("listed %d entries, want <= %d", got, maxListEntries+1)
	}
}
