package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// progressLine 在模型长时间无输出时显示一行“已用时”，不污染管道输出。
// 只在 TTY、非 JSON、非 quiet 时启用；文本流未结束时不动光标。
type progressLine struct {
	out       io.Writer
	enabled   bool
	started   time.Time
	lastEvent time.Time
	textOpen  bool
	drawn     bool
}

func newProgressLine(out io.Writer, enabled bool) *progressLine {
	now := time.Now()
	return &progressLine{out: out, enabled: enabled, started: now, lastEvent: now}
}

func (p *progressLine) clear() {
	if !p.enabled || !p.drawn {
		return
	}
	fmt.Fprint(p.out, "\r"+strings.Repeat(" ", 80)+"\r")
	p.drawn = false
}

func (p *progressLine) beforeEvent(e core.Event) {
	if !p.enabled {
		return
	}
	p.clear()
	p.lastEvent = time.Now()
	if e.Type == core.EvtTextDelta {
		if d, ok := e.Data.(core.TextDeltaData); ok && strings.HasSuffix(d.Text, "\n") {
			p.textOpen = false
		} else {
			p.textOpen = true
		}
		return
	}
	p.textOpen = false
}

func (p *progressLine) tick() {
	if !p.enabled || p.textOpen {
		return
	}
	if time.Since(p.lastEvent) < 2*time.Second {
		return
	}
	elapsed := time.Since(p.started).Truncate(time.Second)
	fmt.Fprintf(p.out, "\r… 已用时 %s · Ctrl+C 中断", elapsed)
	p.drawn = true
}
