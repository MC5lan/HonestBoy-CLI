package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

func newGoProject(t *testing.T, testBody string) *Sandbox {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/hbtest\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package hbtest\nimport \"testing\"\nfunc TestA(t *testing.T) {\n"+testBody+"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sb, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sb.Close() })
	return sb
}

func TestTestArgsDisablesCache(t *testing.T) {
	args := TestArgs()
	found := false
	for _, a := range args {
		if a == "-count=1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("test command must include -count=1: %v", args)
	}
}

func TestVerifyBuildAndTest(t *testing.T) {
	sb := newGoProject(t, "")
	ctx := context.Background()
	build, err := Build(ctx, sb, ".")
	if err != nil || !build.OK || build.Kind != core.EvBuild || build.Provenance != core.ProvObserved {
		t.Fatalf("build: %+v %v", build, err)
	}
	test, err := Test(ctx, sb, ".")
	if err != nil || !test.OK || test.Kind != core.EvTest {
		t.Fatalf("test: %+v %v", test, err)
	}
	vet, err := Vet(ctx, sb, ".")
	if err != nil || !vet.OK || vet.Kind != core.EvVet {
		t.Fatalf("vet: %+v %v", vet, err)
	}
}

func TestVerifyFailingTest(t *testing.T) {
	sb := newGoProject(t, `t.Fatal("boom")`)
	test, err := Test(context.Background(), sb, ".")
	if err != nil {
		t.Fatal(err)
	}
	if test.OK || !strings.Contains(test.Summary, "FAIL") || !strings.Contains(test.Summary, "TestA") {
		t.Fatalf("failing test evidence: %+v", test)
	}
}

func TestExtractSummaryOriginalText(t *testing.T) {
	s := extractSummary("ok example.com/hbtest 0.123s", core.EvBuild, true)
	if s != "ok example.com/hbtest 0.123s" {
		t.Fatalf("summary = %q", s)
	}
	s = extractSummary("x.go:12:2: undefined: foo", core.EvBuild, false)
	if !strings.Contains(s, "undefined: foo") {
		t.Fatalf("summary = %q", s)
	}
}
