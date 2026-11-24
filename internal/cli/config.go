package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		fmt.Fprintf(os.Stdout, "workdir        = %s\n", firstNonEmpty(cfg.Workdir, "(当前目录)"))
		fmt.Fprintf(os.Stdout, "theme          = %s\n", cfg.Theme)
		fmt.Fprintf(os.Stdout, "max_turns      = %d\n", cfg.MaxTurns)
		fmt.Fprintf(os.Stdout, "reasoning      = %s\n", firstNonEmpty(cfg.ReasoningEffort, "(默认)"))
		outTokens := "auto"
		if cfg.MaxOutputTokens > 0 {
			outTokens = formatK(cfg.MaxOutputTokens)
		}
		fmt.Fprintf(os.Stdout, "max_output     = %s\n", outTokens)
		fmt.Fprintf(os.Stdout, "rigor          = %s\n", firstNonEmpty(cfg.Rigor, "normal"))
		fmt.Fprintf(os.Stdout, "context.window = %d (%s)\n", cfg.Context.Window, formatK(cfg.Context.Window))
		fmt.Fprintf(os.Stdout, "approval.write = %s\n", cfg.Approval.Write)
		fmt.Fprintf(os.Stdout, "approval.exec  = %s\n", cfg.Approval.Exec)
		fmt.Fprintf(os.Stdout, "tools.custom_dir = %s\n", firstNonEmpty(cfg.Tools.CustomDir, "(关闭)"))
		fmt.Fprintf(os.Stdout, "web.enabled    = %v\n", cfg.Web.Enabled)
		fmt.Fprintf(os.Stdout, "web.domains    = %s\n", strings.Join(cfg.Web.AllowedDomains, ", "))
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
	case "workdir":
		return c.Workdir, true
	case "theme":
		return c.Theme, true
	case "max_turns":
		return strconv.Itoa(c.MaxTurns), true
	case "reasoning_effort":
		return c.ReasoningEffort, true
	case "rigor":
		return c.Rigor, true
	case "max_output_tokens":
		if c.MaxOutputTokens <= 0 {
			return "auto", true
		}
		return strconv.Itoa(c.MaxOutputTokens), true
	case "context.window", "context_window":
		return strconv.Itoa(c.Context.Window), true
	case "approval.write":
		return c.Approval.Write, true
	case "approval.exec":
		return c.Approval.Exec, true
	case "tools.custom_dir":
		return c.Tools.CustomDir, true
	case "web.enabled":
		return strconv.FormatBool(c.Web.Enabled), true
	case "web.allowed_domains":
		raw, _ := json.Marshal(c.Web.AllowedDomains)
		return string(raw), true
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
	case "workdir":
		abs, err := filepath.Abs(value)
		if err != nil {
			return ErrUsage
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			return fmt.Errorf("%w: 工作区目录不存在: %s", ErrUsage, abs)
		}
		c.Workdir = abs
	case "theme":
		c.Theme = value
	case "max_turns":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return ErrUsage
		}
		c.MaxTurns = n
	case "reasoning_effort":
		switch strings.ToLower(value) {
		case "", "low", "high", "max":
			c.ReasoningEffort = strings.ToLower(value)
		default:
			return ErrUsage
		}
	case "rigor":
		switch strings.ToLower(value) {
		case "normal", "strict":
			c.Rigor = strings.ToLower(value)
		default:
			return ErrUsage
		}
	case "max_output_tokens":
		if strings.EqualFold(value, "auto") {
			c.MaxOutputTokens = 0
			break
		}
		n, ok := parseTokenCount(value)
		if !ok {
			return ErrUsage
		}
		c.MaxOutputTokens = n
	case "context.window", "context_window":
		n, ok := parseTokenCount(value)
		if !ok {
			return ErrUsage
		}
		c.Context.Window = n
	case "approval.write", "approval.exec":
		if value != "auto" && value != "ask" {
			return ErrUsage
		}
		if strings.HasSuffix(key, "write") {
			c.Approval.Write = value
		} else {
			c.Approval.Exec = value
		}
	case "tools.custom_dir":
		c.Tools.CustomDir = strings.TrimSpace(value)
	case "web.enabled":
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return ErrUsage
		}
		c.Web.Enabled = b
	case "web.allowed_domains":
		domains, err := parseDomains(value)
		if err != nil {
			return err
		}
		c.Web.AllowedDomains = domains
	default:
		return ErrUsage
	}
	return nil
}

func parseDomains(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	var domains []string
	if strings.HasPrefix(value, "[") {
		if err := json.Unmarshal([]byte(value), &domains); err != nil {
			return nil, ErrUsage
		}
	} else {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(part), "."))
			if part != "" {
				domains = append(domains, part)
			}
		}
	}
	return domains, nil
}

// parseTokenCount 解析 300k / 600k / 960k / 300000 这类窗口设置。
func parseTokenCount(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	mult := 1
	switch {
	case strings.HasSuffix(s, "k"):
		mult = 1000
		s = strings.TrimSpace(strings.TrimSuffix(s, "k"))
	case strings.HasSuffix(s, "m"):
		mult = 1000000
		s = strings.TrimSpace(strings.TrimSuffix(s, "m"))
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	n *= mult
	if n < 16000 || n > 10000000 {
		return 0, false
	}
	return n, true
}

func formatK(n int) string {
	if n >= 1000 && n%1000 == 0 {
		return strconv.Itoa(n/1000) + "k"
	}
	return strconv.Itoa(n)
}
