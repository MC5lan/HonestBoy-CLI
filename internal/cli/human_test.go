package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func TestHumanEventsFriendly(t *testing.T) {
	tests := []struct {
		ev   core.Event
		want string
	}{
		{core.Event{Type: core.EvtToolRequest, Data: core.ToolRequestData{Tool: "read_file"}}, "读取文件"},
		{core.Event{Type: core.EvtGateAllow}, "检查通过"},
		{core.Event{Type: core.EvtTurnEnd, Data: core.TurnEndData{Terminal: "untested"}}, "结果: 未验证"},
	}
	for _, tc := range tests {
		var buf bytes.Buffer
		if err := WriteHumanEvent(&buf, tc.ev); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), tc.want) {
			t.Fatalf("got %q, want %q", buf.String(), tc.want)
		}
	}
}

func TestFriendlyErrorAuth(t *testing.T) {
	raw := `环境错误: {"error":{"message":"Authentication Fails","type":"invalid_request_error"}}`
	got := friendlyError(raw)
	if !strings.Contains(got, "密钥不对") || strings.Contains(got, `"error"`) {
		t.Fatalf("friendly auth error = %q", got)
	}
}

func TestFriendlyErrorTerminal(t *testing.T) {
	got := FriendlyError(&TerminalError{Terminal: core.TermDenied})
	if !strings.Contains(got, "缺验证证据") {
		t.Fatalf("terminal friendly = %q", got)
	}
}
