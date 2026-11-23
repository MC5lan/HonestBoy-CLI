package cli

import (
	"context"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var interruptCount atomic.Int32

// SetupSignal 处理 Ctrl-C：第一次取消上下文，第二次（3 秒内）立即退出 130。
func SetupSignal(ctx context.Context, cancel context.CancelFunc) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			select {
			case <-ch:
				n := interruptCount.Add(1)
				if n >= 2 {
					os.Exit(130)
				}
				cancel()
				go func() {
					time.Sleep(3 * time.Second)
					interruptCount.Store(0)
				}()
			case <-ctx.Done():
				signal.Stop(ch)
				return
			}
		}
	}()
}
