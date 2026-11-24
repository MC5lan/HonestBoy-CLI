package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// TD05 E5：模型输出乱码时引擎必须在事件边界净化，不崩、不把非法字节写进会话。
func TestEngineInvalidUTF8OutputNoPanic(t *testing.T) {
	bad := string([]byte{0xff, 0xfe, 'a'})
	c := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: bad}, {Text: "中文"}},
	}}
	rs := runEngine(t, newTestEngine(t, c), "随便问一句")
	if rs.terminal == "" {
		t.Fatal("no terminal emitted")
	}
	var text string
	for _, ev := range rs.events {
		d, ok := ev.Data.(core.TextDeltaData)
		if !ok {
			continue
		}
		if !utf8.ValidString(d.Text) {
			t.Fatalf("invalid utf8 leaked into event: %q", d.Text)
		}
		text += d.Text
	}
	if !strings.Contains(text, "\ufffd") || !strings.Contains(text, "中文") {
		t.Fatalf("sanitized text missing: %q", text)
	}
}

// pauseClient 先吐一段乱码，然后挂住；不监听 ctx、也不关 channel，
// 保证引擎只能走 ctx.Done() 分支，断言不被调度顺序影响。
type pauseClient struct {
	sent    chan struct{}
	release chan struct{}
}

func (c *pauseClient) Stream(_ context.Context, _ llm.Request) (<-chan llm.Chunk, <-chan error) {
	chunks := make(chan llm.Chunk, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errs)
		chunks <- llm.Chunk{Text: string([]byte{0xff, 0xfe, 'a'})}
		close(c.sent)
		<-c.release
	}()
	return chunks, errs
}

func TestEngineInvalidUTF8CanBeInterrupted(t *testing.T) {
	c := &pauseClient{sent: make(chan struct{}), release: make(chan struct{})}
	defer close(c.release)
	eng := newTestEngine(t, c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, errs := eng.Run(ctx, "乱码")
	select {
	case <-c.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("engine never consumed the chunk")
	}
	cancel()
	for range events {
	}
	if err := <-errs; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

// 断流后引擎应注入续写提示并在同回合内继续，不丢已输出内容。
func TestEngineResumeAfterDisconnect(t *testing.T) {
	c := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: "前半"}, {Disconnected: true}},
		{{Text: "后半"}},
	}}
	rs := runEngine(t, newTestEngine(t, c), "续写测试")
	var text string
	noticed := false
	for _, ev := range rs.events {
		if d, ok := ev.Data.(core.TextDeltaData); ok {
			text += d.Text
		}
		if d, ok := ev.Data.(core.NoticeData); ok && strings.Contains(d.Text, "连接中断") {
			noticed = true
		}
	}
	if !strings.Contains(text, "前半") || !strings.Contains(text, "后半") {
		t.Fatalf("resume lost text: %q", text)
	}
	if !noticed {
		t.Fatal("missing disconnect notice")
	}
	if len(c.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(c.requests))
	}
	found := false
	for _, m := range c.requests[1].Messages {
		if strings.Contains(m.Content, "从截断处继续") && strings.Contains(m.Content, "前半") {
			found = true
		}
	}
	if !found {
		t.Fatal("resume prompt not injected into second request")
	}
}

// 端点持续断流时必须有上限，不能把 MaxTurns 耗光或假装成功。
func TestEngineResumeRetriesExhausted(t *testing.T) {
	disconnected := llm.Chunk{Disconnected: true}
	c := &scriptedClient{responses: [][]llm.Chunk{
		{{Text: "a"}, disconnected},
		{{Text: "b"}, disconnected},
		{{Text: "c"}, disconnected},
	}}
	rs := runEngine(t, newTestEngine(t, c), "一直断")
	if rs.terminal != core.TermFailed {
		t.Fatalf("terminal = %s, want failed", rs.terminal)
	}
	found := false
	for _, ev := range rs.events {
		if d, ok := ev.Data.(core.ErrorData); ok && strings.Contains(d.Message, "重试后仍未收到完整响应") {
			found = true
		}
	}
	if !found {
		t.Fatal("missing exhausted-resume error event")
	}
}

// TD05 E6：用户输入只有 emoji 时必须被当成普通任务处理，不能 panic 或丢输入。
func TestEngineEmojiOnlyPrompt(t *testing.T) {
	prompt := "👨‍👩‍👧🎉🎉"
	c := &scriptedClient{responses: [][]llm.Chunk{{{Text: "好的"}}}}
	rs := runEngine(t, newTestEngine(t, c), prompt)
	if rs.terminal == "" {
		t.Fatal("no terminal emitted")
	}
	found := false
	for _, ev := range rs.events {
		if d, ok := ev.Data.(core.TextDeltaData); ok && !utf8.ValidString(d.Text) {
			t.Fatalf("invalid utf8 leaked into event: %q", d.Text)
		}
		if d, ok := ev.Data.(core.TurnStartData); ok && strings.Contains(d.Input, prompt) {
			found = true
		}
	}
	if !found {
		t.Fatal("emoji-only prompt was lost")
	}
}
