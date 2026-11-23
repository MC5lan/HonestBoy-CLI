package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

func TestOutsideAccessRejectsModelInventedPath(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(outside, []byte("SECRET"), 0o600)
	args, _ := json.Marshal(map[string]string{"path": outside})
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("r1", "read_file", string(args))},
		{{Text: "好的"}},
	}}
	rs := runEngine(t, newTestEngine(t, client), "读个文件") // 输入里没有路径
	found := false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "未出现在用户输入") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("model-invented outside path must be rejected")
	}
}

func TestOutsideAccessWithUserApproval(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "note.txt")
	_ = os.WriteFile(outside, []byte("USER NOTE"), 0o600)
	args, _ := json.Marshal(map[string]string{"path": outside})
	client := &scriptedClient{responses: [][]llm.Chunk{
		{call("r1", "read_file", string(args))},
		{{Text: "读到了"}},
	}}
	eng := newTestEngine(t, client)
	eng.approvalHandler = func(ctx context.Context, req core.ApprovalRequest) (core.ApprovalDecision, error) {
		if req.Tool != "outside_read" || !req.HighRisk {
			t.Fatalf("unexpected approval: %+v", req)
		}
		return core.ApprovalDecision{Allowed: true}, nil
	}
	rs := runEngine(t, eng, "请读 "+outside)
	gotContent, gotNotice := false, false
	for _, ev := range rs.events {
		if ev.Type == core.EvtNotice {
			if d, ok := ev.Data.(core.NoticeData); ok {
				if strings.Contains(d.Text, "工作区外读取") {
					gotNotice = true
				}
				if strings.Contains(d.Text, "USER NOTE") {
					gotContent = true
				}
			}
		}
	}
	if !gotContent || !gotNotice {
		t.Fatalf("outside read failed: content=%v notice=%v", gotContent, gotNotice)
	}
}
