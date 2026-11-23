package component

// EvidenceList 展示当前回合的证据。T407 复用 core.FormatEvidence。
type EvidenceList struct{ lines []string }

func (e *EvidenceList) Lines() []string { return e.lines }
