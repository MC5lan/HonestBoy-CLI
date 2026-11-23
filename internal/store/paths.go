package store

import (
	"os"
	"path/filepath"
)

// Paths 返回 %LOCALAPPDATA%\HB 下的目录集合。
type Paths struct {
	Root     string
	Sessions string
	Logs     string
}

func DefaultPaths() Paths {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		} else {
			base = os.TempDir()
		}
	}
	root := filepath.Join(base, "HB")
	return Paths{Root: root, Sessions: filepath.Join(root, "sessions"), Logs: filepath.Join(root, "logs")}
}
