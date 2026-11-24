package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

const maxWebBytes = 2 << 20

// WebFetchTool 是显式开启的受限网络读取：域名白名单 + 私网/回环拒绝。
type WebFetchTool struct {
	allowed map[string]bool
	client  *http.Client
}

func NewWebFetchTool(domains []string) *WebFetchTool {
	allowed := make(map[string]bool, len(domains))
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(d, ".")))
		if d != "" {
			allowed[d] = true
		}
	}
	t := &WebFetchTool{allowed: allowed}
	t.client = &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向过多")
			}
			if !t.allowedHost(req.URL.Hostname()) {
				return fmt.Errorf("重定向域名不在白名单: %s", req.URL.Hostname())
			}
			return nil
		},
	}
	return t
}

func (t *WebFetchTool) Name() string { return "web_fetch" }
func (t *WebFetchTool) Description() string {
	return "读取白名单域名的网页/文本（默认关闭，需要审批）"
}
func (t *WebFetchTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"http/https URL"}},"required":["url"]}`)
}
func (t *WebFetchTool) Risk() Risk { return RiskNet }

func (t *WebFetchTool) Run(ctx context.Context, raw json.RawMessage) (Result, error) {
	var a struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.URL) == "" {
		return failResult("参数错误: url 必填")
	}
	u, err := url.Parse(a.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return failResult("只支持 http/https URL")
	}
	if !t.allowedHost(u.Hostname()) {
		return failResult("域名不在白名单: " + u.Hostname())
	}
	if err := rejectPrivateHost(ctx, u.Hostname()); err != nil {
		return failResult(err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return failResult("构造请求失败: " + err.Error())
	}
	req.Header.Set("User-Agent", "hb-webfetch/1")
	resp, err := t.client.Do(req)
	if err != nil {
		return failResult("请求失败: " + err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxWebBytes+1))
	if err != nil {
		return failResult("读取失败: " + err.Error())
	}
	truncated := len(body) > maxWebBytes
	if truncated {
		body = body[:maxWebBytes]
	}
	output := fmt.Sprintf("HTTP %d %s\n%s", resp.StatusCode, resp.Header.Get("Content-Type"), string(body))
	if truncated {
		output += "\n… 响应超过 2MB，已截断"
	}
	return Result{
		Output: output, OK: resp.StatusCode >= 200 && resp.StatusCode < 300,
		Meta:       map[string]string{"status": fmt.Sprint(resp.StatusCode), "url": u.String()},
		Provenance: core.ProvObserved,
	}, nil
}

func (t *WebFetchTool) allowedHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if t.allowed[host] {
		return true
	}
	for d := range t.allowed {
		if strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func rejectPrivateHost(ctx context.Context, host string) error {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS 解析失败: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("DNS 无结果")
	}
	for _, ip := range ips {
		if ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsUnspecified() {
			return fmt.Errorf("拒绝访问私网/回环地址: %s", ip.IP)
		}
	}
	return nil
}

// RegisterWebFetch 仅在显式 enabled 时注册；白名单为空直接报错。
func RegisterWebFetch(r *Registry, enabled bool, domains []string) error {
	if !enabled {
		return nil
	}
	if len(domains) == 0 {
		return fmt.Errorf("web.enabled=true 时必须配置 web.allowed_domains")
	}
	return r.Register(NewWebFetchTool(domains))
}
