package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ClaimRecord 是模型断言的轨迹记录（文档 §3.6.3）。
// Grounded 字段是坦荡体系的核心观测点：模型说这句话时是否有证据支撑。
type ClaimRecord struct {
	V         int    `json:"v"`
	TS        string `json:"ts"`
	Text      string `json:"text"`
	Grounded  bool   `json:"grounded"`
	GateState string `json:"gate_state"`
}

// TraceEvent 是轨迹事件（transition / evidence / claim / halt）。
type TraceEvent struct {
	V      int             `json:"v"`
	TS     string          `json:"ts"`
	Type   string          `json:"type"`
	From   string          `json:"from,omitempty"`
	To     string          `json:"to,omitempty"`
	Reason string          `json:"reason,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// Tracer 追加写轨迹到 JSONL（文档 §3.6.4：批量刷盘，不解析历史）。
type Tracer struct {
	dir    string
	file   *os.File
	claims []ClaimRecord
}

func NewTracer(dir string) *Tracer {
	return &Tracer{dir: dir}
}

func (t *Tracer) path() string { return filepath.Join(t.dir, "trace.jsonl") }

// RecordClaim 记录一条断言及其是否有证据支撑。
func (t *Tracer) RecordClaim(text, gateState string, grounded bool) {
	c := ClaimRecord{V: 1, TS: time.Now().UTC().Format(time.RFC3339Nano),
		Text: text, Grounded: grounded, GateState: gateState}
	t.claims = append(t.claims, c)
	b, _ := json.Marshal(c)
	t.appendJSON(b)
}

// RecordTransition 记录状态转移。
func (t *Tracer) RecordTransition(from, to, reason string) {
	e := TraceEvent{V: 1, TS: time.Now().UTC().Format(time.RFC3339Nano),
		Type: "transition", From: from, To: to, Reason: reason}
	b, _ := json.Marshal(e)
	t.appendJSON(b)
}

// RecordHalt 记录主动中断（不计失败）。
func (t *Tracer) RecordHalt(reason string) {
	e := TraceEvent{V: 1, TS: time.Now().UTC().Format(time.RFC3339Nano),
		Type: "halt", Reason: reason}
	b, _ := json.Marshal(e)
	t.appendJSON(b)
}

func (t *Tracer) appendJSON(b []byte) {
	if err := os.MkdirAll(t.dir, 0o700); err != nil {
		return
	}
	if t.file == nil {
		f, err := os.OpenFile(t.path(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		t.file = f
	}
	_, _ = t.file.Write(append(b, '\n'))
}

// UCR 返回未证实断言率（Ungrounded Claim Rate）——坦荡水位计（文档 §6.1）。
func (t *Tracer) UCR() float64 {
	if len(t.claims) == 0 {
		return 0
	}
	ungrounded := 0
	for _, c := range t.claims {
		if !c.Grounded {
			ungrounded++
		}
	}
	return float64(ungrounded) / float64(len(t.claims))
}

// Close 关闭轨迹文件。
func (t *Tracer) Close() error {
	if t.file != nil {
		return t.file.Close()
	}
	return nil
}
