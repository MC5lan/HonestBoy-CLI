//go:build !windows

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withFakeRg(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rg")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// rg 正常返回 0 条命中时不能再跑 Go 兜底，否则大仓库会白扫两遍。
func TestSearchRgNoMatchDoesNotFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withFakeRg(t, "#!/bin/sh\nexit 1\n")
	sb, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sb.Close() }()

	res, err := NewSearchTool(sb).Run(context.Background(), json.RawMessage(`{"pattern":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Meta["hits"]; got != "0" || res.Output != "" {
		t.Fatalf("rg no-match fell back to walk: hits=%q output=%q", got, res.Output)
	}
}

func TestSearchRgMatchParsed(t *testing.T) {
	withFakeRg(t, `#!/bin/sh
printf '%s\n' '{"type":"match","data":{"path":{"text":"a.txt"},"line_number":1,"lines":{"text":"needle\n"}}}'
exit 0
`)
	sb, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sb.Close() }()

	res, err := NewSearchTool(sb).Run(context.Background(), json.RawMessage(`{"pattern":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Meta["hits"]; got != "1" || !strings.Contains(res.Output, "needle") {
		t.Fatalf("rg match not parsed: hits=%q output=%q", got, res.Output)
	}
}

// rg 真出错（exit 2）才允许回退 Go 遍历。
func TestSearchRgErrorFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withFakeRg(t, "#!/bin/sh\nexit 2\n")
	sb, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sb.Close() }()

	res, err := NewSearchTool(sb).Run(context.Background(), json.RawMessage(`{"pattern":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Meta["hits"]; got != "1" {
		t.Fatalf("rg error should fall back, hits=%q output=%q", got, res.Output)
	}
}
