package cli

import (
	"context"
	"os"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// pingModel 发一个极小的真实请求，验证端点/密钥/模型是否可用。
func pingModel(cfg store.Config, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	key := os.Getenv(firstNonEmpty(cfg.APIKeyEnv, "HB_API_KEY"))
	client := llm.NewClient(cfg.Protocol, cfg.BaseURL, key, cfg.Model)
	chunks, errs := client.Stream(ctx, llm.Request{
		Model:     cfg.Model,
		Messages:  []llm.Message{{Role: "user", Content: "ping"}},
		MaxTokens: 16,
	})
	for range chunks {
	}
	if err := <-errs; err != nil {
		return err
	}
	return ctx.Err()
}
