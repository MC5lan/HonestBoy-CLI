package cli

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	releaseAPI  = "https://api.github.com/repos/MC5lan/HonestBoy-CLI/releases/latest"
	downloadTTL = 2 * time.Minute
)

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// RunUpgrade 实现 hb upgrade：检查最新 release、校验 SHA256、替换自身二进制。
// --check 只报告；--api 覆盖 API 根地址；--dest 覆盖目标路径（测试与自定义安装用）。
func RunUpgrade(opts GlobalOptions, args []string) error {
	var checkOnly, force bool
	api := releaseAPI
	dest := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			checkOnly = true
		case "--force":
			force = true
		case "--api":
			if v, ok := takeValue(args, &i); ok {
				api = v
			} else {
				return ErrUsage
			}
		case "--dest":
			if v, ok := takeValue(args, &i); ok {
				dest = v
			} else {
				return ErrUsage
			}
		default:
			return ErrUsage
		}
	}
	if dest == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		dest = exe
	}

	rel, err := fetchRelease(api)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	fmt.Fprintf(os.Stdout, "当前版本 %s，最新版本 %s\n", Version, latest)
	if checkOnly {
		if compareVersion(latest, Version) > 0 {
			fmt.Fprintln(os.Stdout, "有新版本可用：hb upgrade")
		} else {
			fmt.Fprintln(os.Stdout, "已是最新版本")
		}
		return nil
	}
	if compareVersion(latest, Version) <= 0 && !force {
		fmt.Fprintln(os.Stdout, "已是最新版本（--force 可强制重装）")
		return nil
	}

	assetName := fmt.Sprintf("hb_%s_%s_%s.zip", latest, runtime.GOOS, runtime.GOARCH)
	var assetURL, sumsURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			assetURL = a.URL
		case "checksums.txt":
			sumsURL = a.URL
		}
	}
	if assetURL == "" || sumsURL == "" {
		return fmt.Errorf("%w: 发布缺少资产 %s 或 checksums.txt", ErrEnvironment, assetName)
	}

	fmt.Fprintln(os.Stdout, "下载校验和…")
	sums, err := httpGet(sumsURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	wantHash, err := findChecksum(string(sums), assetName)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	fmt.Fprintln(os.Stdout, "下载 "+assetName+"…")
	archive, err := httpGet(assetURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	got := sha256.Sum256(archive)
	if gotHex := hex.EncodeToString(got[:]); gotHex != wantHash {
		return fmt.Errorf("%w: SHA256 不匹配（期望 %s，实际 %s）", ErrEnvironment, wantHash, gotHex)
	}
	bin, err := extractBinary(archive)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	if err := replaceBinary(dest, bin); err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	fmt.Fprintf(os.Stdout, "已更新到 %s（%s）。重启后生效。\n", latest, dest)
	return nil
}

func fetchRelease(api string) (releaseInfo, error) {
	raw, err := httpGet(api)
	if err != nil {
		return releaseInfo{}, err
	}
	var rel releaseInfo
	if err := json.Unmarshal(raw, &rel); err != nil {
		return releaseInfo{}, fmt.Errorf("release JSON 解析失败: %w", err)
	}
	if rel.TagName == "" {
		return releaseInfo{}, fmt.Errorf("release 缺少 tag_name")
	}
	return rel, nil
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: downloadTTL}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hb-upgrade/"+Version)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && strings.Contains(url, "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 128<<20))
}

func findChecksum(sums, asset string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt 里没有 %s", asset)
}

func extractBinary(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("zip 解析失败: %w", err)
	}
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != "hb" && base != "hb.exe" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 200<<20))
	}
	return nil, fmt.Errorf("压缩包里没有 hb 二进制")
}

// replaceBinary 原子替换目标文件；Windows 下先把运行中的旧文件改名。
func replaceBinary(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".hb-new-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Rename(tmpName, dest); err != nil {
			os.Remove(tmpName)
			return err
		}
		return nil
	}
	old := dest + ".old"
	_ = os.Remove(old)
	renamed := false
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			os.Remove(tmpName)
			return fmt.Errorf("无法重命名旧二进制（是否正在运行？）: %w", err)
		}
		renamed = true
	}
	if err := os.Rename(tmpName, dest); err != nil {
		if renamed {
			_ = os.Rename(old, dest)
		}
		os.Remove(tmpName)
		return err
	}
	// 运行中的旧进程会占用 .old，删不掉就留给下次启动清理。
	_ = os.Remove(old)
	return nil
}

// compareVersion 比较点分版本号，忽略 v 前缀与预发布后缀；越大越新。
func compareVersion(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	var out []int
	for _, part := range strings.Split(v, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}
