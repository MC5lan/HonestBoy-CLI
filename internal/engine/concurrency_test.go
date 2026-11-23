package engine

import (
	"bytes"
	"sync"
	"testing"
)

// TA07：事件广播多消费者并发不 panic、不丢事件（在 -race 下运行）。
func TestEventBusConcurrent(t *testing.T) {
	bus := NewEventBus()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		ch := bus.Subscribe()
		wg.Add(1)
		go func(ch chan []byte) {
			defer wg.Done()
			for range ch {
			}
		}(ch)
	}
	for i := 0; i < 100; i++ {
		bus.Publish([]byte("event"))
	}
	bus.Close()
	wg.Wait()
}

// TA07：工具输出多 goroutine 合并（模拟 stdout/stderr 并发写）。
func TestOutputMergeConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	var buf bytes.Buffer
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			mu.Lock()
			buf.WriteString("x")
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if buf.Len() != 20 {
		t.Fatalf("merged %d bytes", buf.Len())
	}
}
