package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/errs"
)

// TC07：错误码表与代码登记一致；改代码不改文档则测试红。
func TestErrorCodesDocumented(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("errors.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for code := range errs.Errors {
		if !strings.Contains(doc, string(code)) {
			t.Fatalf("error code %s missing from docs/errors.md", code)
		}
	}
}

// TC07：退出码表与实现一致。
func TestExitCodesDocumented(t *testing.T) {
	raw, err := os.ReadFile("exit-codes.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, code := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "130"} {
		if !strings.Contains(doc, "| "+code+" |") {
			t.Fatalf("exit code %s missing from docs/exit-codes.md", code)
		}
	}
}
