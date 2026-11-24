package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMigratesMissingFields(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	dir := filepath.Join(root, "HB")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"model":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Migrated || cfg.SchemaVersion != 1 || cfg.APIKeyEnv == "" {
		t.Fatalf("cfg = %+v", cfg)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"_migrated": true`) || !strings.Contains(string(raw), `"model": "old"`) {
		t.Fatalf("migrated config = %s", raw)
	}
}
