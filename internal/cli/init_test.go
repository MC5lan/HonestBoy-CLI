package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitGuide(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("HB_TEST_KEY", "")
	old := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = old }()
	// 工作区=回车(当前), 协议=2(anthropic), base_url=回车, model=回车, keyenv=回车
	go func() {
		w.WriteString("\n2\n\n\n\n")
		w.Close()
	}()

	err := RunInit(GlobalOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "HB", "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"protocol": "anthropic"`, "base_url", "api_key_env", "HB_API_KEY"} {
		if !strings.Contains(text, want) {
			t.Fatalf("config missing %q: %s", want, text)
		}
	}
}

func TestNeedsSetup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	if !needsSetup() {
		t.Fatal("fresh install should need setup")
	}
}

func TestRunErrorMentionsInit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("HB_API_KEY", "")
	err := RunRun(GlobalOptions{}, []string{"test"})
	if err == nil || !strings.Contains(err.Error(), "hb init") {
		t.Fatalf("error should guide to hb init: %v", err)
	}
}
