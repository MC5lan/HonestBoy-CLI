package cli

import (
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
