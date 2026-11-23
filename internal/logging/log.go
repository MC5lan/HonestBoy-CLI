package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	mu     sync.RWMutex
	logger *slog.Logger
	closer io.Closer
)

// Init 初始化 JSON 结构化日志，写入 %LOCALAPPDATA%\HB\logs\hb-YYYYMMDD.log。
// 所有输出经 redactWriter，密钥不可能进日志文件。
func Init(debug bool, dir string) error {
	if dir == "" {
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "HB", "logs")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := filepath.Join(dir, "hb-"+time.Now().Format("20060102")+".log")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	w := &redactWriter{inner: f}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	mu.Lock()
	if closer != nil {
		_ = closer.Close()
	}
	logger, closer = slog.New(h), f
	mu.Unlock()
	return nil
}

// L 返回全局 logger（未初始化时返回默认处理器，不 panic）。
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// Close 关闭日志文件。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if closer != nil {
		err := closer.Close()
		closer, logger = nil, nil
		return err
	}
	return nil
}

// redactWriter 在所有写入路径强制脱敏，不靠调用方自觉。
type redactWriter struct {
	inner io.Writer
}

func (w *redactWriter) Write(p []byte) (int, error) {
	clean := Redact(string(p))
	if clean != string(p) {
		// 长度可能变化，返回原始长度以让 slog 认为写入成功。
		_, err := io.WriteString(w.inner, clean)
		return len(p), err
	}
	return w.inner.Write(p)
}

// With 给 logger 绑定会话/回合/序号上下文（T901）。
func With(sessionID, turnID string, seq uint64) *slog.Logger {
	return L().With("session_id", sessionID, "turn_id", turnID, "seq", seq)
}

// DumpEvent 把事件以诊断日志记录（与产品事件分离）。
func DumpEvent(ev any) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	L().Debug("event", "payload", strings.TrimSpace(fmt.Sprint(string(b))))
}
