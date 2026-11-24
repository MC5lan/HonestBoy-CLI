package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
	"github.com/MC5lan/HonestBoy-CLI/internal/tools"
)

func abstainCall(id, reason string) llm.Chunk {
	return call(id, "declare_untested", `{"reason":"`+reason+`","not_done":["测试"]}`)
}

// 续接同一会话时，第二回合的系统提示必须包含第一回合的历史。
func TestResumeCarriesPreviousTurnIntoContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module resume\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	paths := store.Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}

	client1 := &scriptedClient{responses: [][]llm.Chunk{{abstainCall("u1", "第一轮没有验证")}}}
	sb1, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	eng1 := New(Config{
		Workdir: dir, Model: "test", Client: client1,
		Registry: tools.NewDefaultRegistry(sb1, tools.NewReadGuard(), "cmd"),
		Store:    store.NewSessionStore(paths), MaxTurns: 5,
	})
	res1 := runEngine(t, eng1, "第一轮任务：请只回答，不改文件")
	sb1.Close()
	sessionID := ""
	for _, ev := range res1.events {
		if ev.Type == core.EvtTurnStart {
			sessionID = ev.SessionID
		}
	}
	if sessionID == "" {
		t.Fatal("first run did not emit session id")
	}

	client2 := &scriptedClient{responses: [][]llm.Chunk{{abstainCall("u2", "第二轮没有验证")}}}
	sb2, err := tools.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer sb2.Close()
	eng2 := New(Config{
		Workdir: dir, Model: "test", Client: client2, ResumeID: sessionID,
		Registry: tools.NewDefaultRegistry(sb2, tools.NewReadGuard(), "cmd"),
		Store:    store.NewSessionStore(paths), MaxTurns: 5,
	})
	runEngine(t, eng2, "第二轮任务：接着上一轮")
	if len(client2.requests) == 0 || len(client2.requests[0].Messages) == 0 {
		t.Fatal("second run made no request")
	}
	system := client2.requests[0].Messages[0].Content
	if !strings.Contains(system, "之前回合") {
		t.Fatalf("second run missing history section:\n%s", system)
	}
	if !strings.Contains(system, "第一轮任务") {
		t.Fatalf("second run missing previous user input:\n%s", system)
	}
	// 续接后的证据链必须包含两个回合，且仍然完整。
	sess, err := store.NewSessionStore(paths).Load(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Receipts) != 2 {
		t.Fatalf("receipts = %d, want 2", len(sess.Receipts))
	}
	if at, err := core.VerifyChain(sess.Receipts); err != nil {
		t.Fatalf("receipt chain broken at %d: %v", at, err)
	}
}
