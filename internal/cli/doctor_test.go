package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

func TestProbeEndpointFallsBackToGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	code, err := probeEndpoint(srv.URL)
	if err != nil || code != http.StatusOK {
		t.Fatalf("probeEndpoint = %d, %v", code, err)
	}
}

func TestProbeEndpointReachableOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	code, err := probeEndpoint(srv.URL)
	if err != nil || code != http.StatusUnauthorized {
		t.Fatalf("probeEndpoint = %d, %v", code, err)
	}
}

func TestKeyLooksLikeSecret(t *testing.T) {
	for _, s := range []string{"sk-live-abc", "sk-test-abc", "012345678901234567890123456789012345678901"} {
		if !keyLooksLikeSecret(s) {
			t.Fatalf("%q should look like a secret", s)
		}
	}
	for _, s := range []string{"HB_API_KEY", "DEEPSEEK_KEY", "LOCAL_KEY"} {
		if keyLooksLikeSecret(s) {
			t.Fatalf("%q should be a variable name", s)
		}
	}
}

func TestIsLocalBaseURL(t *testing.T) {
	for _, s := range []string{"http://localhost:8080/v1", "http://127.0.0.1:11434/v1", "http://[::1]:8080"} {
		if !isLocalBaseURL(s) {
			t.Fatalf("%q should be local", s)
		}
	}
	for _, s := range []string{"https://api.openai.com/v1", "https://example.com"} {
		if isLocalBaseURL(s) {
			t.Fatalf("%q should not be local", s)
		}
	}
}

func findCheck(checks []checkResult, name string) checkResult {
	for _, c := range checks {
		if c.name == name {
			return c
		}
	}
	return checkResult{}
}

func TestDoctorChecksMissingConfig(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	cfg := store.DefaultConfig()
	checks := doctorChecks(cfg, nil, t.TempDir(), nil, func(string) string { return "" }, false)
	for _, name := range []string{"模型配置", "密钥变量", "模型端点"} {
		if c := findCheck(checks, name); c.level != "fail" {
			t.Fatalf("%s level = %q, want fail (%+v)", name, c.level, c)
		}
	}
}

func TestDoctorChecksEndpointReachable(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	cfg := store.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Model = "test-model"
	checks := doctorChecks(cfg, nil, t.TempDir(), nil, func(string) string { return "secret" }, false)
	if c := findCheck(checks, "模型端点"); c.level != "ok" {
		t.Fatalf("endpoint = %+v, want ok", c)
	}
	if c := findCheck(checks, "密钥变量"); c.level != "ok" {
		t.Fatalf("key = %+v, want ok", c)
	}
}

func TestDoctorChecksSecretAsEnvName(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	cfg := store.DefaultConfig()
	cfg.APIKeyEnv = "sk-live-oops"
	checks := doctorChecks(cfg, nil, t.TempDir(), nil, func(string) string { return "" }, false)
	if c := findCheck(checks, "密钥变量"); c.level != "fail" {
		t.Fatalf("secret env name = %+v, want fail", c)
	}
}
