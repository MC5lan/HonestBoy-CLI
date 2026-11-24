package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// recoverCrash 在引擎主循环最外层捕获 panic，写崩溃日志并尝试 flush 会话（T904）。
func recoverCrash(e *Engine) {
	if r := recover(); r != nil {
		dir := filepath.Join(e.store.Paths.Logs, "crash")
		_ = os.MkdirAll(dir, 0o700)
		name := filepath.Join(dir, "crash-"+time.Now().Format("20060102-150405.000")+".log")
		content := fmt.Sprintf("panic: %v\n\n%s", r, debug.Stack())
		_ = os.WriteFile(name, []byte(content), 0o600)
		if e.session != nil {
			e.session.Emit(core.EvtError, core.ErrorData{Code: "HB-IO-001", Message: "内部错误：" + fmt.Sprint(r)})
		}
		fmt.Fprintf(os.Stderr, "hb: 内部错误，崩溃日志已写入 %s\n", name)
	}
}
