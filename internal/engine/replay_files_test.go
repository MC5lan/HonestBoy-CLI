package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

const replayDataDir = "../../testdata/sessions"

// 生成真实会话文件：UPDATE_REPLAY=1 go test ./internal/engine -run TestGenerateReplayFiles
func TestGenerateReplayFiles(t *testing.T) {
	if os.Getenv("UPDATE_REPLAY") != "1" {
		t.Skip("set UPDATE_REPLAY=1 to regenerate testdata/sessions")
	}
	if err := os.MkdirAll(replayDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range replayCases {
		eng := newTestEngine(t, &scriptedClient{responses: tc.script})
		if tc.setup != nil {
			tc.setup(t, eng)
		}
		_runReplayCase(t, eng, tc)
		entries, err := os.ReadDir(eng.store.Paths.Sessions)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".jsonl") || strings.HasSuffix(entry.Name(), ".lock") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(eng.store.Paths.Sessions, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(replayDataDir, tc.name+".jsonl"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
}

func TestFileReplayTerminalsAndChains(t *testing.T) {
	ss := store.NewSessionStore(store.Paths{Sessions: replayDataDir})
	for _, tc := range replayCases {
		t.Run(tc.name, func(t *testing.T) {
			sess, err := ss.Load(tc.name)
			if err != nil {
				t.Fatalf("load replay %s: %v", tc.name, err)
			}
			if len(sess.Events) == 0 {
				t.Fatal("empty replay")
			}
			terminal := ""
			ended := false
			for _, ev := range sess.Events {
				switch ev.Type {
				case core.EvtTurnEnd:
					if d, ok := ev.Data.(core.TurnEndData); ok {
						terminal = d.Terminal
					}
				case core.EvtSessionEnd:
					ended = true
				}
			}
			if terminal != string(tc.want) {
				t.Fatalf("terminal = %q, want %q", terminal, tc.want)
			}
			if !ended {
				t.Fatal("missing session_end")
			}
			if len(sess.Receipts) == 0 {
				t.Fatal("missing receipt")
			}
			if at, err := core.VerifyChain(sess.Receipts); err != nil {
				t.Fatalf("receipt chain broken at %d: %v", at, err)
			}
		})
	}
}

func _runReplayCase(t *testing.T, eng *Engine, tc replayCase) {
	t.Helper()
	rs := runEngine(t, eng, tc.prompt)
	if rs.terminal != tc.want {
		t.Fatalf("generate replay %s: terminal=%s want=%s", tc.name, rs.terminal, tc.want)
	}
}
