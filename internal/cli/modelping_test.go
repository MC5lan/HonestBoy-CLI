package cli

import (
	"testing"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm/testserver"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

func pingConfig(url string) store.Config {
	cfg := store.DefaultConfig()
	cfg.BaseURL = url
	cfg.Model = "test-model"
	cfg.APIKeyEnv = "HB_PING_TEST_KEY"
	return cfg
}

func TestPingModel(t *testing.T) {
	srv := testserver.New(t, testserver.Script{Responses: [][]string{{
		`{"choices":[{"delta":{"content":"ok"},"finish_reason":null}]}`,
	}}})
	t.Setenv("HB_PING_TEST_KEY", "secret")
	if err := pingModel(pingConfig(srv.URL), 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestPingModelAuthError(t *testing.T) {
	srv := testserver.New(t, testserver.Script{StatusCodes: []int{401}})
	t.Setenv("HB_PING_TEST_KEY", "secret")
	if err := pingModel(pingConfig(srv.URL), 5*time.Second); err == nil {
		t.Fatal("401 should fail the connection test")
	}
}
