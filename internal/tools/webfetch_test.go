package tools

import (
	"context"
	"testing"
)

func TestWebFetchDomainAllowlist(t *testing.T) {
	w := NewWebFetchTool([]string{"example.com", "docs.example.org"})
	for _, host := range []string{"example.com", "api.example.com", "docs.example.org"} {
		if !w.allowedHost(host) {
			t.Fatalf("%s should be allowed", host)
		}
	}
	for _, host := range []string{"evil.com", "example.com.evil.com", "notexample.com"} {
		if w.allowedHost(host) {
			t.Fatalf("%s should be denied", host)
		}
	}
}

func TestWebFetchRejectsLoopback(t *testing.T) {
	if err := rejectPrivateHost(context.Background(), "localhost"); err == nil {
		t.Fatal("localhost must be rejected")
	}
}

func TestRegisterWebFetchRequiresAllowlist(t *testing.T) {
	r := NewRegistry()
	if err := RegisterWebFetch(r, true, nil); err == nil {
		t.Fatal("enabled without allowlist must fail")
	}
	if err := RegisterWebFetch(r, false, nil); err != nil {
		t.Fatal(err)
	}
}
