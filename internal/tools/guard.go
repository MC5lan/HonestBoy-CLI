package tools

import "crypto/sha256"

// ReadGuard 记录本会话读过的文件哈希，防止盲改与外部并发改动。
type ReadGuard struct{ seen map[string][32]byte }

func NewReadGuard() *ReadGuard { return &ReadGuard{seen: make(map[string][32]byte)} }

func (g *ReadGuard) MarkRead(path string, content []byte) {
	sum := sha256.Sum256(content)
	g.seen[path] = sum
}

// Check 写前校验：未读过 → ErrNeedsReadFirst；已被外部改动 → ErrChangedSinceRead。
func (g *ReadGuard) Check(path string, current []byte) error {
	want, ok := g.seen[path]
	if !ok {
		return ErrNeedsReadFirst
	}
	got := sha256.Sum256(current)
	if got != want {
		return ErrChangedSinceRead
	}
	return nil
}

func (g *ReadGuard) HasRead(path string) bool {
	_, ok := g.seen[path]
	return ok
}
