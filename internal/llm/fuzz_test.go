package llm

import (
	"bytes"
	"strings"
	"testing"
)

// FuzzParseSSE 对 SSE 解析器做模糊测试（TA08）；任何输入不得 panic 或死循环。
func FuzzParseSSE(f *testing.F) {
	f.Add([]byte("data: {\"a\":1}\n\n"))
	f.Add([]byte("event: x\r\ndata: y\r\n\r\n"))
	f.Add([]byte(": heartbeat\n\ndata:z\n"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, b []byte) {
		_ = ParseSSE(bytes.NewReader(b), func(_, _ []byte) error { return nil })
	})
}

func TestParseSSELongLineNoPanic(t *testing.T) {
	input := "data: " + strings.Repeat("x", maxSSELine+100) + "\n\n"
	_ = ParseSSE(strings.NewReader(input), func(_, _ []byte) error { return nil })
}
