package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func TestProgressLineDisabled(t *testing.T) {
	var buf bytes.Buffer
	p := newProgressLine(&buf, false)
	p.tick()
	p.clear()
	if buf.Len() != 0 {
		t.Fatalf("disabled progress wrote %q", buf.String())
	}
}

func TestProgressLineIdleAndText(t *testing.T) {
	var buf bytes.Buffer
	p := newProgressLine(&buf, true)
	p.started = time.Now().Add(-3 * time.Second)
	p.lastEvent = time.Now().Add(-3 * time.Second)
	p.tick()
	if !strings.Contains(buf.String(), "已用时") {
		t.Fatalf("idle progress missing: %q", buf.String())
	}

	// 正文没有换行时不要覆盖当前行。
	p.beforeEvent(core.Event{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "半行"}})
	buf.Reset()
	p.lastEvent = time.Now().Add(-3 * time.Second)
	p.tick()
	if buf.Len() != 0 {
		t.Fatalf("progress drew over open text: %q", buf.String())
	}

	// 正文换行后可以继续显示。
	p.beforeEvent(core.Event{Type: core.EvtTextDelta, Data: core.TextDeltaData{Text: "结束\n"}})
	p.lastEvent = time.Now().Add(-3 * time.Second)
	p.tick()
	if !strings.Contains(buf.String(), "已用时") {
		t.Fatalf("progress did not resume after newline: %q", buf.String())
	}
}
