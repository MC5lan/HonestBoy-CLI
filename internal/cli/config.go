package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunConfig 实现 hb config get/set/list/path。
func RunConfig(opts GlobalOptions, args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}
	sub := args[0]
	if sub == "path" {
		fmt.Fprintln(os.Stdout, store.ConfigPath())
		return nil
	}
	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	switch sub {
	case "list":
		fmt.Fprintf(os.Stdout, "schema_version = %d\n", cfg.SchemaVersion)
		fmt.Fprintf(os.Stdout, "protocol       = %s\n", cfg.Protocol)
		fmt.Fprintf(os.Stdout, "model          = %s\n", cfg.Model)
		fmt.Fprintf(os.Stdout, "base_url       = %s\n", cfg.BaseURL)
		fmt.Fprintf(os.Stdout, "api_key        = ***（来自 %s）\n", firstNonEmpty(cfg.APIKeyEnv, "HB_API_KEY"))
		fmt.Fprintf(os.Stdout, "api_key_env    = %s\n", cfg.APIKeyEnv)
		fmt.Fprintf(os.Stdout, "shell          = %s\n", cfg.Shell)
		fmt.Fprintf(os.Stdout, "theme          = %s\n", cfg.Theme)
		fmt.Fprintf(os.Stdout, "max_turns      = %d\n", cfg.MaxTurns)
		fmt.Fprintf(os.Stdout, "approval.write = %s\n", cfg.Approval.Write)
		fmt.Fprintf(os.Stdout, "approval.exec  = %s\n", cfg.Approval.Exec)
		return nil
	case "get":
		if len(args) < 2 {
			return ErrUsage
		}
		v, ok := configField(cfg, args[1])
		if !ok {
			return ErrUsage
		}
		fmt.Fprintln(os.Stdout, v)
		return nil
	case "set":
		if len(args) < 3 {
			return ErrUsage
		}
		key, value := args[1], strings.Join(args[2:], " ")
		if strings.EqualFold(key, "api_key") {
			return fmt.Errorf("%w: 密钥只能放环境变量，配置里写 api_key_env", ErrUsage)
		}
		if err := setConfigField(&cfg, key, value); err != nil {
			return err
		}
		if err := store.Save(cfg); err != nil {
			return fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		fmt.Fprintf(os.Stdout, "已设置 %s\n", key)
		return nil
	default:
		return ErrUsage
	}
}

func configField(c store.Config, key string) (string, bool) {
	switch strings.ToLower(key) {
	case "protocol":
		return c.Protocol, true
	case "model":
		return c.Model, true
	case "base_url":
		return c.BaseURL, true
	case "api_key_env":
		return c.APIKeyEnv, true
	case "shell":
		return c.Shell, true
	case "theme":
		return c.Theme, true
	case "max_turns":
		return strconv.Itoa(c.MaxTurns), true
	case "approval.write":
		return c.Approval.Write, true
	case "approval.exec":
		return c.Approval.Exec, true
	default:
		return "", false
	}
}

func setConfigField(c *store.Config, key, value string) error {
	switch strings.ToLower(key) {
	case "protocol":
		switch value {
		case "openai-chat", "anthropic", "responses":
			c.Protocol = value
		default:
			return ErrUsage
		}
	case "model":
		c.Model = value
	case "base_url":
		c.BaseURL = value
	case "api_key_env":
		c.APIKeyEnv = value
	case "shell":
		c.Shell = value
	case "theme":
		c.Theme = value
	case "max_turns":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return ErrUsage
		}
		c.MaxTurns = n
	case "approval.write", "approval.exec":
		if value != "auto" && value != "ask" {
			return ErrUsage
		}
		if strings.HasSuffix(key, "write") {
			c.Approval.Write = value
		} else {
			c.Approval.Exec = value
		}
	default:
		return ErrUsage
	}
	return nil
}
