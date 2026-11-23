package engine

import "sync"

// EventBus 广播带 seq 的事件给多个消费者。
type EventBus struct {
	mu   sync.RWMutex
	subs map[chan []byte]struct{}
}

func NewEventBus() *EventBus { return &EventBus{subs: make(map[chan []byte]struct{})} }

func (b *EventBus) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBus) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// Close 关闭所有订阅 channel，供消费者退出。
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		close(ch)
	}
	b.subs = make(map[chan []byte]struct{})
}

func (b *EventBus) Publish(payload []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- payload:
		default:
		}
	}
}
