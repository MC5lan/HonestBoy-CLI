package tools

import (
	"context"
	"strings"
	"testing"
)

func TestApplyPatchAddUpdateDelete(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	tool := NewApplyPatchTool(sb, guard)
	ctx := context.Background()

	patch := `*** Begin Patch
*** Add File: hello.txt
+hello
+world
*** Update File: hello.txt
@@
 hello
-world
+HONEST
*** End Patch`
	r, err := tool.Run(ctx, mustArgs(t, map[string]string{"patch": patch}))
	if err != nil || !r.OK {
		t.Fatalf("patch: %+v %v", r, err)
	}
	data, err := sb.ReadFile("hello.txt")
	if err != nil || string(data) != "hello\nHONEST\n" {
		t.Fatalf("content = %q, %v", data, err)
	}
	if r.Meta["sha256"] == "" {
		t.Fatal("missing diff hash")
	}

	del := `*** Begin Patch
*** Delete File: hello.txt
*** End Patch`
	r, err = tool.Run(ctx, mustArgs(t, map[string]string{"patch": del}))
	if err != nil || !r.OK {
		t.Fatalf("delete: %+v %v", r, err)
	}
	if _, err := sb.Stat("hello.txt"); err == nil {
		t.Fatal("file should be deleted")
	}
}

func TestApplyPatchAmbiguousAndMalformed(t *testing.T) {
	sb, _ := newTempSandbox(t)
	guard := NewReadGuard()
	tool := NewApplyPatchTool(sb, guard)
	ctx := context.Background()
	_ = NewWriteFileTool(sb, guard).Run

	write := NewWriteFileTool(sb, guard)
	_, _ = write.Run(ctx, mustArgs(t, map[string]any{"path": "a.txt", "content": "x\nx\n"}))
	ambiguous := `*** Begin Patch
*** Update File: a.txt
@@
-x
+y
*** End Patch`
	r, _ := tool.Run(ctx, mustArgs(t, map[string]string{"patch": ambiguous}))
	if r.OK || !strings.Contains(r.Output, "匹配到 2 处") {
		t.Fatalf("ambiguous: %+v", r)
	}
	bad := `just text`
	r, _ = tool.Run(ctx, mustArgs(t, map[string]string{"patch": bad}))
	if r.OK || !strings.Contains(r.Output, "Begin Patch") {
		t.Fatalf("malformed: %+v", r)
	}
}
