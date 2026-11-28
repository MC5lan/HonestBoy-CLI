package render

import (
	"os"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`https?://[^\s\x1b]+`)

// LinkSupported 只在已知支持 OSC 8 的终端启用；其余终端保留可复制的 URL。
func LinkSupported() bool {
	switch strings.ToLower(os.Getenv("TERM_PROGRAM")) {
	case "apple_terminal":
		return false
	case "iterm.app", "vscode", "wezterm", "windows_terminal":
		return true
	}
	return os.Getenv("WT_SESSION") != "" || os.Getenv("WEZTERM_PANE") != ""
}

// WrapURLs 给渲染后的文本加 OSC 8 超链接；不支持时原样返回。
func WrapURLs(s string) string {
	if !LinkSupported() || strings.Contains(s, "\x1b]8;;") {
		return s
	}
	return urlPattern.ReplaceAllStringFunc(s, func(u string) string {
		return "\x1b]8;;" + u + "\x1b\\" + u + "\x1b]8;;\x1b\\"
	})
}
