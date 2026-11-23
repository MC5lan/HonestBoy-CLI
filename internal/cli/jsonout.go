package cli

import (
	"encoding/json"
	"io"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// jsonFlusher 让每行写完立即 Flush，保证 --json 可实时被 jq 消费。
type jsonFlusher struct {
	w io.Writer
}

func (f *jsonFlusher) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if flusher, ok := f.w.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
	return n, err
}

// WriteJSONEvent 输出一行 JSON 事件（Event 与 §4.3 完全一致）。
func WriteJSONEvent(w io.Writer, e core.Event) error {
	enc := json.NewEncoder(&jsonFlusher{w: w})
	enc.SetEscapeHTML(false)
	return enc.Encode(e)
}
