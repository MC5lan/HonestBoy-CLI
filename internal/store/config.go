package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config 是配置文件结构；api_key 字段永不出现，只保存环境变量名。
type Config struct {
	SchemaVersion int    `json:"schema_version"`
	Model         string `json:"model"`
	// Protocol 选择模型协议：openai-chat（默认）| anthropic | responses。
	// 为空时由 llm.DetectProtocol 根据 base_url 推断。
	Protocol  string `json:"protocol,omitempty"`
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
	Shell     string `json:"shell"`
	Theme     string `json:"theme"`
	MaxTurns  int    `json:"max_turns"`
	Timeout   string `json:"timeout"`
	// Workdir 是默认工作区（沙箱边界）；空表示使用进程当前目录。
	Workdir string `json:"workdir,omitempty"`
	// ReasoningEffort 是思考强度：low | high | max；空为提供方默认。
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Approval        struct {
		Write    string `json:"write"`
		Exec     string `json:"exec"`
		Net      string `json:"net"`
		HighRisk string `json:"high_risk"`
	} `json:"approval"`
	Ignore  []string `json:"ignore"`
	Context struct {
		// Window 是模型上下文窗口（token）；支持 300k / 600k / 960k。
		// 0 表示使用默认 960k（由 DefaultConfig 填充）。
		Window      int     `json:"window"`
		BudgetRatio float64 `json:"budget_ratio"`
		KeepRecent  int     `json:"keep_recent_turns"`
		HBMDLimit   int     `json:"hb_md_limit"`
	} `json:"context"`
	Tools struct {
		ShellTimeoutMS   int `json:"shell_timeout_ms"`
		OutputLimitBytes int `json:"output_limit_bytes"`
		SearchMaxResults int `json:"search_max_results"`
	} `json:"tools"`
	UI struct {
		Width              int  `json:"width"`
		ShowTimestamps     bool `json:"show_timestamps"`
		CollapseToolBlocks bool `json:"collapse_tool_blocks"`
	} `json:"ui"`
}

func DefaultConfig() Config {
	c := Config{SchemaVersion: 1, Protocol: "openai-chat", APIKeyEnv: "HB_API_KEY", Shell: "cmd", Theme: "auto", MaxTurns: 20, Timeout: "10m"}
	c.Context.BudgetRatio = 0.7
	c.Context.KeepRecent = 2
	c.Context.HBMDLimit = 16384
	c.Context.Window = 960000
	c.Tools.ShellTimeoutMS = 120000
	c.Tools.OutputLimitBytes = 1048576
	c.Tools.SearchMaxResults = 200
	c.UI.Width = 100
	c.UI.CollapseToolBlocks = true
	return c
}

// ConfigPath 返回配置文件路径。
func ConfigPath() string {
	return filepath.Join(DefaultPaths().Root, "config.json")
}

// Load 读取配置；缺字段由 DefaultConfig 补齐。配置里禁止出现 api_key 字段。
func Load() (Config, error) {
	c := DefaultConfig()
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return c, err
	}
	if _, ok := fields["api_key"]; ok {
		return c, errors.New("config must not contain api_key; use api_key_env")
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.SchemaVersion > 1 {
		return c, errors.New("config schema too new")
	}
	// 迁移标记：缺字段被默认值补齐。
	if c.SchemaVersion < 1 {
		c.SchemaVersion = 1
	}
	return c, nil
}

// Save 原子写入配置，权限 0600。
func Save(c Config) error {
	root := DefaultPaths().Root
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := ConfigPath() + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ConfigPath())
}
