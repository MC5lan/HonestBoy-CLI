package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/logging"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunGC 清理过期会话与日志（T908）。
// 默认只列出（dry-run），加 --yes 才真正删除。
func RunGC(opts GlobalOptions, args []string) error {
	dry := !opts.Yes
	for _, a := range args {
		if a == "--dry-run" {
			dry = true
		}
	}
	paths := store.DefaultPaths()
	ss := store.NewSessionStore(paths)

	metas, err := ss.List()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	sessionCutoff := time.Now().AddDate(0, 0, -90)
	var oldSessions []store.SessionMeta
	for _, m := range metas {
		if m.ModifiedAt.Before(sessionCutoff) {
			oldSessions = append(oldSessions, m)
		}
	}

	logCutoff := time.Now().AddDate(0, 0, -14)
	var oldLogs []string
	if entries, err := os.ReadDir(paths.Logs); err == nil {
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(logCutoff) {
				oldLogs = append(oldLogs, filepath.Join(paths.Logs, e.Name()))
			}
		}
	}

	size, _ := ss.DirSize()
	fmt.Fprintf(os.Stdout, "会话目录占用 %.1f MB，共 %d 个会话\n", float64(size)/(1<<20), len(metas))
	if len(oldSessions) == 0 && len(oldLogs) == 0 {
		fmt.Fprintln(os.Stdout, "没有需要清理的过期数据（会话 >90 天、日志 >14 天）")
		return nil
	}
	if len(oldSessions) > 0 {
		fmt.Fprintf(os.Stdout, "\n会话超过 90 天（%d 个）：\n", len(oldSessions))
		for _, m := range oldSessions {
			fmt.Fprintf(os.Stdout, "  %s  %s\n", m.ID, m.ModifiedAt.Format("2006-01-02"))
		}
	}
	if len(oldLogs) > 0 {
		fmt.Fprintf(os.Stdout, "\n日志超过 14 天（%d 个）：\n", len(oldLogs))
		for _, f := range oldLogs {
			fmt.Fprintf(os.Stdout, "  %s\n", filepath.Base(f))
		}
	}
	if dry {
		fmt.Fprintln(os.Stdout, "\n这是预演（dry-run）。加 --yes 真正删除。")
		return nil
	}
	deleted := 0
	for _, m := range oldSessions {
		if err := ss.Delete(m.ID); err == nil {
			deleted++
		}
	}
	for _, f := range oldLogs {
		if err := os.Remove(f); err == nil {
			deleted++
		}
	}
	fmt.Fprintf(os.Stdout, "\n已删除 %d 项。\n", deleted)
	return nil
}

// RunExport 导出会话为可分享的 jsonl（T909），--redact 脱敏。
func RunExport(opts GlobalOptions, args []string) error {
	if len(args) < 1 {
		return ErrUsage
	}
	sessionID := args[0]
	redact := false
	for _, a := range args {
		if a == "--redact" {
			redact = true
		}
	}
	ss := store.NewSessionStore(store.DefaultPaths())
	sess, err := ss.Load(sessionID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	return writeExport(os.Stdout, sess, redact)
}

// writeExport 把会话写成每行一个 JSON 对象，便于流式脱敏与管道消费。
func writeExport(w io.Writer, sess *core.Session, redact bool) error {
	var b strings.Builder
	meta, err := json.Marshal(map[string]string{"kind": "meta", "id": sess.ID, "cwd": sess.Cwd})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	metaLine := string(meta)
	if redact {
		metaLine = redactLine(metaLine)
	}
	b.WriteString(metaLine + "\n")
	for _, ev := range sess.Events {
		raw, err := jsonMarshal(ev)
		if err != nil {
			continue
		}
		line := string(raw)
		if redact {
			line = redactLine(line)
		}
		b.WriteString(line + "\n")
	}
	fmt.Fprint(w, b.String())
	return nil
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
func redactLine(line string) string     { return logging.Redact(line) }
