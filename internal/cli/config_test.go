package cli

import (
	"path/filepath"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

func TestConfigReasoningEffort(t *testing.T) {
	c := store.DefaultConfig()
	if err := setConfigField(&c, "reasoning_effort", "low"); err != nil {
		t.Fatal(err)
	}
	if c.ReasoningEffort != "low" {
		t.Fatalf("ReasoningEffort = %q", c.ReasoningEffort)
	}
	if v, ok := configField(c, "reasoning_effort"); !ok || v != "low" {
		t.Fatalf("get = %q, %v", v, ok)
	}
	if err := setConfigField(&c, "reasoning_effort", "bogus"); err == nil {
		t.Fatal("invalid effort should be rejected")
	}
}

func TestConfigOutputBudget(t *testing.T) {
	c := store.DefaultConfig()
	if c.MaxOutputTokens != 0 {
		t.Fatalf("default max output = %d, want auto(0)", c.MaxOutputTokens)
	}
	if v, ok := configField(c, "max_output_tokens"); !ok || v != "auto" {
		t.Fatalf("get auto = %q, %v", v, ok)
	}
	if err := setConfigField(&c, "max_output_tokens", "128k"); err != nil {
		t.Fatal(err)
	}
	if c.MaxOutputTokens != 128000 {
		t.Fatalf("max output = %d", c.MaxOutputTokens)
	}
	if err := setConfigField(&c, "max_output_tokens", "auto"); err != nil {
		t.Fatal(err)
	}
	if c.MaxOutputTokens != 0 {
		t.Fatalf("auto reset = %d", c.MaxOutputTokens)
	}
	if err := setConfigField(&c, "max_output_tokens", "bogus"); err == nil {
		t.Fatal("invalid output budget should be rejected")
	}
}

func TestConfigRigor(t *testing.T) {
	c := store.DefaultConfig()
	if c.Rigor != "normal" {
		t.Fatalf("default rigor = %q", c.Rigor)
	}
	if err := setConfigField(&c, "rigor", "strict"); err != nil {
		t.Fatal(err)
	}
	if c.Rigor != "strict" {
		t.Fatalf("rigor = %q", c.Rigor)
	}
	if v, ok := configField(c, "rigor"); !ok || v != "strict" {
		t.Fatalf("get = %q, %v", v, ok)
	}
	if err := setConfigField(&c, "rigor", "bogus"); err == nil {
		t.Fatal("invalid rigor should be rejected")
	}
}

func TestConfigWorkdir(t *testing.T) {
	c := store.DefaultConfig()
	dir := t.TempDir()
	if err := setConfigField(&c, "workdir", dir); err != nil {
		t.Fatal(err)
	}
	if c.Workdir != dir {
		t.Fatalf("Workdir = %q", c.Workdir)
	}
	if v, ok := configField(c, "workdir"); !ok || v != dir {
		t.Fatalf("get = %q, %v", v, ok)
	}
	if err := setConfigField(&c, "workdir", filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing directory should be rejected")
	}
}

func TestConfigContextWindowPresets(t *testing.T) {
	c := store.DefaultConfig()
	if c.Context.Window != 960000 {
		t.Fatalf("default window = %d", c.Context.Window)
	}
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"300k", 300000},
		{"600K", 600000},
		{"960k", 960000},
		{"300000", 300000},
	} {
		if err := setConfigField(&c, "context.window", tc.value); err != nil {
			t.Fatalf("%s: %v", tc.value, err)
		}
		if c.Context.Window != tc.want {
			t.Fatalf("%s = %d, want %d", tc.value, c.Context.Window, tc.want)
		}
	}
	for _, bad := range []string{"600", "0", "abc", "20m"} {
		if err := setConfigField(&c, "context.window", bad); err == nil {
			t.Fatalf("%q should be rejected", bad)
		}
	}
	if v, ok := configField(c, "context.window"); !ok || v != "300000" {
		t.Fatalf("get = %q, %v", v, ok)
	}
}
