package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

type checkResult struct {
	name, detail, fix string
	level             string // ok | warn | fail
}

// RunDoctor 环境自检：只诊断；--fix 仅重建无法读取的配置。
func RunDoctor(opts GlobalOptions, args []string) error {
	fix := false
	for _, arg := range args {
		if arg == "--fix" {
			fix = true
		}
	}
	cfg, cfgErr := store.Load()
	fixed := false
	if cfgErr != nil && fix {
		cfg = store.DefaultConfig()
		if err := store.Save(cfg); err == nil {
			cfgErr, fixed = nil, true
		}
	}
	wd, wdErr := ResolveWorkdir("", cfg.Workdir)
	checks := doctorChecks(cfg, cfgErr, wd, wdErr, os.Getenv, fixed)
	printDoctor(checks)
	return nil
}

func doctorChecks(cfg store.Config, cfgErr error, wd string, wdErr error, getenv func(string) string, fixed bool) []checkResult {
	var checks []checkResult
	checks = append(checks, checkResult{name: "版本", detail: VersionString(), level: "ok"})

	cfgDetail := store.ConfigPath()
	if fixed {
		cfgDetail += "（已重建默认配置）"
	}
	if cfgErr != nil {
		checks = append(checks, checkResult{name: "配置文件", detail: cfgErr.Error(), level: "fail",
			fix: "检查 JSON 格式与路径权限，或运行 hb doctor --fix"})
	} else {
		checks = append(checks, checkResult{name: "配置文件", detail: cfgDetail, level: "ok"})
	}

	protocol := firstNonEmpty(cfg.Protocol, "openai-chat")
	model := firstNonEmpty(cfg.Model, "(未设置)")
	if cfg.Model == "" || cfg.BaseURL == "" {
		checks = append(checks, checkResult{name: "模型配置", detail: protocol + " · " + model, level: "fail",
			fix: "运行 hb init 设置端点与模型"})
	} else {
		checks = append(checks, checkResult{name: "模型配置", detail: protocol + " · " + model, level: "ok"})
	}

	keyEnv := firstNonEmpty(cfg.APIKeyEnv, "HB_API_KEY")
	switch {
	case keyLooksLikeSecret(keyEnv):
		checks = append(checks, checkResult{name: "密钥变量", detail: keyEnv, level: "fail",
			fix: "api_key_env 应是变量名（如 HB_API_KEY），不是 sk- 密钥；运行 hb init 修正"})
	case getenv(keyEnv) != "":
		checks = append(checks, checkResult{name: "密钥变量", detail: keyEnv + " 已设置", level: "ok"})
	case isLocalBaseURL(cfg.BaseURL):
		checks = append(checks, checkResult{name: "密钥变量", detail: keyEnv + " 未设置（本地网关可忽略）", level: "warn",
			fix: "如果端点需要密钥，设置 " + keyEnv + " 或运行 set-key.ps1"})
	default:
		checks = append(checks, checkResult{name: "密钥变量", detail: keyEnv + " 未设置", level: "fail",
			fix: "设置环境变量 " + keyEnv + "，或运行 set-key.ps1"})
	}

	if cfg.BaseURL == "" {
		checks = append(checks, checkResult{name: "模型端点", detail: "(未设置)", level: "fail",
			fix: "运行 hb init，或 hb config set base_url <url>"})
	} else if status, err := probeEndpoint(cfg.BaseURL); err != nil {
		checks = append(checks, checkResult{name: "模型端点", detail: cfg.BaseURL + " · " + err.Error(), level: "fail",
			fix: "检查 base_url、HTTPS_PROXY、代理与防火墙；修复后重跑 hb doctor"})
	} else if status >= 500 {
		checks = append(checks, checkResult{name: "模型端点", detail: fmt.Sprintf("%s · HTTP %d", cfg.BaseURL, status), level: "warn",
			fix: "端点可达但服务端报错，稍后重试或换端点"})
	} else {
		checks = append(checks, checkResult{name: "模型端点", detail: fmt.Sprintf("%s · HTTP %d", cfg.BaseURL, status), level: "ok"})
	}

	if wdErr != nil {
		checks = append(checks, checkResult{name: "工作区", detail: wdErr.Error(), level: "fail",
			fix: "运行 hb config set workdir <存在的目录>"})
	} else {
		tmp := filepath.Join(wd, ".hb-doctor-"+fmt.Sprint(time.Now().UnixNano()))
		werr := os.WriteFile(tmp, []byte("x"), 0o600)
		if werr == nil {
			_ = os.Remove(tmp)
		}
		switch {
		case werr != nil:
			checks = append(checks, checkResult{name: "工作区", detail: wd, level: "fail",
				fix: "检查目录权限，换一个有写权限的项目目录"})
		case isHomeDirPath(wd):
			checks = append(checks, checkResult{name: "工作区", detail: wd + "（用户主目录）", level: "warn",
				fix: "建议在具体项目目录运行 hb；主目录会跳过检查点与仓库地图"})
		default:
			checks = append(checks, checkResult{name: "工作区", detail: wd, level: "ok"})
		}
	}

	sessions := store.DefaultPaths().Sessions
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		checks = append(checks, checkResult{name: "会话目录", detail: sessions, level: "fail",
			fix: "检查 %LOCALAPPDATA%\\HB 的权限"})
	} else {
		tmp := filepath.Join(sessions, ".hb-doctor-"+fmt.Sprint(time.Now().UnixNano()))
		werr := os.WriteFile(tmp, []byte("x"), 0o600)
		if werr == nil {
			_ = os.Remove(tmp)
		}
		if werr != nil {
			checks = append(checks, checkResult{name: "会话目录", detail: sessions, level: "fail",
				fix: "检查会话目录权限"})
		} else {
			checks = append(checks, checkResult{name: "会话目录", detail: sessions, level: "ok"})
		}
	}

	isTTY := false
	if info, err := os.Stdout.Stat(); err == nil {
		isTTY = info.Mode()&os.ModeCharDevice != 0
	}
	checks = append(checks, checkResult{name: "终端", detail: fmt.Sprintf("TTY=%v · theme=%s", isTTY, firstNonEmpty(cfg.Theme, "auto")), level: "ok",
		fix: "非 TTY 时自动关闭颜色；需要交互请用 hb chat"})

	if _, err := exec.LookPath("rg"); err == nil {
		checks = append(checks, checkResult{name: "ripgrep", detail: "可用", level: "ok"})
	} else {
		checks = append(checks, checkResult{name: "ripgrep", detail: "未找到", level: "warn",
			fix: "搜索会降级为内置遍历；大仓库建议安装：winget install BurntSushi.ripgrep.MSVC（或 scoop/apt install ripgrep）"})
	}
	return checks
}

func printDoctor(checks []checkResult) {
	fmt.Fprintf(os.Stdout, "%-12s %-6s %-44s %s\n", "检查项", "结果", "详情", "建议")
	ok, warn, fail := 0, 0, 0
	var next []string
	seen := map[string]bool{}
	for _, c := range checks {
		mark := "✓"
		switch c.level {
		case "warn":
			mark, warn = "!", warn+1
		case "fail":
			mark, fail = "✗", fail+1
		default:
			ok++
		}
		fmt.Fprintf(os.Stdout, "%-12s %-6s %-44s %s\n", c.name, mark, oneLine(c.detail, 42), c.fix)
		if c.fix != "" && c.level != "ok" && !seen[c.fix] {
			seen[c.fix] = true
			next = append(next, c.fix)
		}
	}
	fmt.Fprintf(os.Stdout, "\n%d 项通过，%d 项警告，%d 项失败\n", ok, warn, fail)
	if len(next) > 0 {
		fmt.Fprintln(os.Stdout, "\n建议下一步：")
		for _, s := range next {
			fmt.Fprintln(os.Stdout, "- "+s)
		}
	}
}

// probeEndpoint 只判断端点是否可达：HEAD 不行就 GET，任何 HTTP 响应都算可达。
func probeEndpoint(baseURL string) (int, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	do := func(method string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(context.Background(), method, baseURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "hb-doctor")
		return client.Do(req)
	}
	resp, err := do(http.MethodHead)
	if err != nil || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented || resp.StatusCode >= 500 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		resp, err = do(http.MethodGet)
	}
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func keyLooksLikeSecret(s string) bool {
	return strings.HasPrefix(strings.ToLower(s), "sk-") || len(s) > 40
}

func isLocalBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasSuffix(strings.ToLower(u.Hostname()), ".local")
}

func isHomeDirPath(p string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	return samePathForDoctor(home, p)
}

func samePathForDoctor(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	if os.PathSeparator == '\\' {
		return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
	}
	return filepath.Clean(aa) == filepath.Clean(bb)
}
