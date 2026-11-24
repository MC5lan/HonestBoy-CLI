package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func BenchmarkLoadThousandEvents(b *testing.B) {
	root := b.TempDir()
	paths := Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
	ss := NewSessionStore(paths)
	sess := core.Session{ID: "s_bench", Cwd: root, CreatedAt: time.Now().UTC()}
	if err := ss.Create(sess); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		ev := sess.Emit(core.EvtNotice, core.NoticeData{Level: "info", Text: strings.Repeat("x", 80)})
		if err := ss.AppendEvent(ev); err != nil {
			b.Fatal(err)
		}
	}
	if err := ss.Unlock(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := NewSessionStore(paths).Load("s_bench"); err != nil {
			b.Fatal(err)
		}
	}
}
