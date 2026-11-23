package cli

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func makeZip(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.1.9", 1},
		{"v0.2.0", "0.2.0", 0},
		{"1.0.0", "dev", 1},
		{"dev", "1.0.0", -1},
		{"2.0.0-beta", "2.0.0", 0},
	}
	for _, c := range cases {
		if got := compareVersion(c.a, c.b); got != c.want {
			t.Fatalf("compareVersion(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

// upgradeServer 提供 release JSON、checksums.txt 与 zip 资产。
func upgradeServer(t *testing.T, zipData []byte, hashOverride string) *httptest.Server {
	t.Helper()
	assetName := fmt.Sprintf("hb_9.9.9_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(zipData)
	hash := hex.EncodeToString(sum[:])
	if hashOverride != "" {
		hash = hashOverride
	}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		rel := map[string]any{
			"tag_name": "v9.9.9",
			"assets": []map[string]string{
				{"name": assetName, "browser_download_url": srv.URL + "/asset.zip"},
				{"name": "checksums.txt", "browser_download_url": srv.URL + "/checksums.txt"},
			},
		}
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hash, assetName)
	})
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipData)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestUpgradeInstalls 验证下载 → SHA256 校验 → 替换目标文件的完整链路。
func TestUpgradeInstalls(t *testing.T) {
	newBin := []byte("new-binary-content")
	srv := upgradeServer(t, makeZip(t, "hb.exe", newBin), "")
	dest := filepath.Join(t.TempDir(), "hb.exe")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = old })

	if err := RunUpgrade(GlobalOptions{}, []string{"--api", srv.URL + "/latest", "--dest", dest}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBin) {
		t.Fatalf("dest not replaced: %q", got)
	}
}

// 校验和不匹配必须中止，目标文件保持原样。
func TestUpgradeChecksumMismatch(t *testing.T) {
	srv := upgradeServer(t, makeZip(t, "hb.exe", []byte("evil")), strings.Repeat("0", 64))
	dest := filepath.Join(t.TempDir(), "hb.exe")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := RunUpgrade(GlobalOptions{}, []string{"--api", srv.URL + "/latest", "--dest", dest})
	if err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatalf("err = %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatalf("dest must be untouched, got %q", got)
	}
}

func TestUpgradeCheckOnlyDoesNotWrite(t *testing.T) {
	srv := upgradeServer(t, makeZip(t, "hb.exe", []byte("new")), "")
	dest := filepath.Join(t.TempDir(), "hb.exe")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = old })
	if err := RunUpgrade(GlobalOptions{}, []string{"--api", srv.URL + "/latest", "--dest", dest, "--check"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatalf("--check must not write, got %q", got)
	}
}

// 已是新版本时默认不下载。
func TestUpgradeAlreadyLatest(t *testing.T) {
	srv := upgradeServer(t, makeZip(t, "hb.exe", []byte("new")), "")
	dest := filepath.Join(t.TempDir(), "hb.exe")
	_ = os.WriteFile(dest, []byte("old"), 0o755)
	old := Version
	Version = "9.9.9"
	t.Cleanup(func() { Version = old })
	if err := RunUpgrade(GlobalOptions{}, []string{"--api", srv.URL + "/latest", "--dest", dest}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatalf("same version must not replace, got %q", got)
	}
}

func TestUpgradeBadReleaseJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	err := RunUpgrade(GlobalOptions{}, []string{"--api", srv.URL + "/latest", "--dest", filepath.Join(t.TempDir(), "hb")})
	if err == nil {
		t.Fatal("expected error for invalid release JSON")
	}
}
