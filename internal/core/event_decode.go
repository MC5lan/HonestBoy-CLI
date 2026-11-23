package core

import (
	"encoding/json"
	"fmt"
	"time"
)

// UnmarshalEvent 按 Type 把 Data 分派到具体结构，禁止 map[string]any 满天飞。
func UnmarshalEvent(raw []byte) (Event, error) {
	var wire struct {
		Seq       uint64          `json:"seq"`
		SessionID string          `json:"session_id"`
		TurnID    string          `json:"turn_id,omitempty"`
		Type      EventType       `json:"type"`
		Time      json.RawMessage `json:"time"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Event{}, err
	}
	ev := Event{Seq: wire.Seq, SessionID: wire.SessionID, TurnID: wire.TurnID, Type: wire.Type}
	if len(wire.Time) > 0 {
		var s string
		if err := json.Unmarshal(wire.Time, &s); err == nil {
			if parsed, perr := time.Parse(time.RFC3339Nano, s); perr == nil {
				ev.Time = parsed
			}
		}
	}
	data, err := decodeEventData(wire.Type, wire.Data)
	if err != nil {
		return Event{}, err
	}
	ev.Data = data
	return ev, nil
}

func decodeEventData(t EventType, raw []byte) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var target any
	switch t {
	case EvtTurnStart:
		target = &TurnStartData{}
	case EvtTextDelta:
		target = &TextDeltaData{}
	case EvtToolRequest:
		target = &ToolRequestData{}
	case EvtApprovalNeeded:
		target = &ApprovalData{}
	case EvtToolStart:
		target = &ToolStartData{}
	case EvtToolEnd:
		target = &ToolEndData{}
	case EvtToolDenied:
		target = &ToolDeniedData{}
	case EvtNotice:
		target = &NoticeData{}
	case EvtGateAllow:
		target = &GateAllowData{}
	case EvtGateDeny:
		target = &GateDenyData{}
	case EvtAbstain:
		target = &AbstainData{}
	case EvtTurnEnd:
		target = &TurnEndData{}
	case EvtDiffReview:
		target = &DiffReviewData{}
	case EvtUsage:
		target = &UsageData{}
	case EvtError:
		target = &ErrorData{}
	default:
		return nil, fmt.Errorf("unknown event type %q", t)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, err
	}
	// Data 字段存值而非指针，消费方类型断言更直接。
	switch v := target.(type) {
	case *TurnStartData:
		return *v, nil
	case *TextDeltaData:
		return *v, nil
	case *ToolRequestData:
		return *v, nil
	case *ApprovalData:
		return *v, nil
	case *ToolStartData:
		return *v, nil
	case *ToolEndData:
		return *v, nil
	case *ToolDeniedData:
		return *v, nil
	case *NoticeData:
		return *v, nil
	case *GateAllowData:
		return *v, nil
	case *GateDenyData:
		return *v, nil
	case *AbstainData:
		return *v, nil
	case *TurnEndData:
		return *v, nil
	case *DiffReviewData:
		return *v, nil
	case *UsageData:
		return *v, nil
	case *ErrorData:
		return *v, nil
	}
	return nil, nil
}
