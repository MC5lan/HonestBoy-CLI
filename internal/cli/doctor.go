package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

type checkResult struct {
	name, detail, fix string
	ok                bool
}

// RunDoctor 环境自检：只诊断，不修改配置（除临时文件即刻删除）。
func RunDoctor(opts GlobalOptions, args []string) error {
	cfg, cfgErr := store.Load()
	var checks []checkResult

	checks = append(checks, checkResult{
		name: "Go 运行时", detail: runtime.Version(),
		ok:  strings.Contains(runtime.Version(), "go1.2") || strings.Contains(runtime.Version(), "go1.3"),
		fix: "需要 Go 1.27+",
	})
	checks = append(checks, checkResult{
		name: "配置可读", detail: store.ConfigPath(), ok: cfgErr == nil,
		fix: "检查路径权限与 JSON 格式，或 hb config list",
	})
	keyName := firstNonEmpty(cfg.APIKeyEnv, "HB_API_KEY")
	checks = append(checks, checkResult{
		name: "模型密钥", detail: keyName, ok: os.Getenv(keyName) != "" || cfg.BaseURL != "",
		fix: "set " + keyName + "=... 或配置 base_url 指向本地网关",
	})
	if cfg.BaseURL != "" {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Head(cfg.BaseURL)
		if resp != nil {
			resp.Body.Close()
		}
		checks = append(checks, checkResult{
			name: "网络", detail: cfg.BaseURL, ok: err == nil,
			fix: "检查 HTTPS_PROXY / 代理 / 防火墙",
		})
	}
	wd, wderr := ResolveWorkdir("", cfg.Workdir)
	if wderr != nil {
		wd = cfg.Workdir
	}
	tmp := filepath.Join(wd, ".hb-doctor-"+fmt.Sprint(time.Now().UnixNano()))
	werr := os.WriteFile(tmp, []byte("x"), 0o600)
	if werr == nil {
		_ = os.Remove(tmp)
	}
	checks = append(checks, checkResult{
		name: "工作区可写", detail: wd, ok: werr == nil, fix: "检查目录权限",
	})
	if _, err := exec.LookPath("rg"); err == nil {
		checks = append(checks, checkResult{name: "rg", detail: "可用", ok: true})
	} else {
		checks = append(checks, checkResult{name: "rg", detail: "未找到", ok: false, fix: "搜索会降级为内置遍历，功能可用但更慢"})
	}
	info, statErr := os.Stdout.Stat()
	isTTY := statErr == nil && (info.Mode()&os.ModeCharDevice) != 0
	checks = append(checks, checkResult{
		name: "终端", detail: fmt.Sprintf("TTY=%v", isTTY), ok: true,
		fix: "非 TTY 时自动关闭颜色；必要时加 --no-color",
	})

	fmt.Fprintf(os.Stdout, "%-14s %-6s %-40s %s\n", "检查项", "结果", "详情", "建议")
	failed := 0
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark, failed = "✗", failed+1
		}
		fmt.Fprintf(os.Stdout, "%-14s %-6s %-40s %s\n", c.name, mark, oneLine(c.detail, 38), c.fix)
	}
	fmt.Fprintf(os.Stdout, "\n%d 项通过，%d 项失败\n", len(checks)-failed, failed)
	return nil
}
