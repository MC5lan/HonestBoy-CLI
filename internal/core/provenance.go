package core

// Provenance 描述一条信息的来源。四值封闭，不许扩展。
type Provenance uint8

const (
	ProvUnknown  Provenance = iota // 不知道从哪来的
	ProvAssumed                    // 假设，没有验证
	ProvInferred                   // 推理得出，逻辑上成立但未证实
	ProvObserved                   // 观察到的确定事实（命令输出、文件内容、测试结果）
)

func (p Provenance) String() string {
	switch p {
	case ProvObserved:
		return "observed"
	case ProvInferred:
		return "inferred"
	case ProvAssumed:
		return "assumed"
	default:
		return "unknown"
	}
}

// CanGround 判断该来源能否作为“完成”的根据。
// 只有 observed 可以。这是整个诚实机制的第一道闸。
func (p Provenance) CanGround() bool { return p == ProvObserved }

// ParseProvenance 解析来源字符串，未知值报错不兜底。
func ParseProvenance(s string) (Provenance, error) {
	switch s {
	case "observed":
		return ProvObserved, nil
	case "inferred":
		return ProvInferred, nil
	case "assumed":
		return ProvAssumed, nil
	case "unknown":
		return ProvUnknown, nil
	default:
		return ProvUnknown, ErrUnknownProvenance
	}
}
