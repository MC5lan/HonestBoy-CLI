package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

const maxEvidenceRaw = 8 << 10

var failTestRe = regexp.MustCompile(`--- FAIL: (\w+)`)

// Build 是 EvBuild 的唯一来源。
func Build(ctx context.Context, sb *Sandbox, target string) (core.Evidence, error) {
	return runVerify(ctx, sb, core.EvBuild, target, projectCommand(sb, "go build", "go build ./...")...)
}

// Test 是 EvTest 的唯一来源；必须带 -count=1 禁用缓存。
func Test(ctx context.Context, sb *Sandbox, target string) (core.Evidence, error) {
	return runVerify(ctx, sb, core.EvTest, target, projectCommand(sb, "go test", "go test ./... -count=1")...)
}

// Vet 是 EvVet 的唯一来源。
func Vet(ctx context.Context, sb *Sandbox, target string) (core.Evidence, error) {
	return runVerify(ctx, sb, core.EvVet, target, projectCommand(sb, "go vet", "go vet ./...")...)
}

// TestArgs 返回测试命令参数，供单测断言 -count=1 存在。
func TestArgs() []string { return []string{"go", "test", "./...", "-count=1"} }

func projectCommand(sb *Sandbox, npmCmd, goCmd string) []string {
	if _, err := sb.Stat("go.mod"); err == nil {
		return strings.Fields(goCmd)
	}
	if _, err := sb.Stat("package.json"); err == nil {
		cmd := strings.Fields(npmCmd)
		if npmCmd == "go build" {
			cmd = []string{"npm", "run", "build"}
		} else if npmCmd == "go test" {
			cmd = []string{"npm", "test"}
		}
		return cmd
	}
	return strings.Fields(goCmd)
}

func runVerify(ctx context.Context, sb *Sandbox, kind core.EvidenceKind, target string, argv ...string) (core.Evidence, error) {
	started := time.Now().UTC()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = sb.Workdir()
	cmd.Env = filteredEnv()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	exitCode := 0
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return core.Evidence{}, err
		}
	}
	output := buf.String()
	raw := output
	if len(raw) > maxEvidenceRaw {
		raw = raw[:maxEvidenceRaw]
	}
	ev := core.Evidence{
		ID:         "ev_" + evidenceID(kind, target, started.UnixNano()),
		Kind:       kind,
		Target:     target,
		OK:         exitCode == 0,
		ExitCode:   exitCode,
		Summary:    extractSummary(output, kind, exitCode == 0),
		Raw:        raw,
		StartedAt:  started,
		Duration:   time.Since(started).Milliseconds(),
		Level:      core.EvidenceLevelVerifier,
		Ran:        true,
		Provenance: core.ProvObserved,
	}
	// 原文哈希在运行的验证器上计算；未运行的错误路径不产证据。
	ev.RawHash = ev.ComputeRawHash()
	return ev, nil
}

// extractSummary 取验证器原文摘要，不改写结论只做截断与失败名提取。
func extractSummary(output string, kind core.EvidenceKind, ok bool) string {
	if ok {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) > 0 && lines[0] != "" {
			return truncateRunes(lines[0], 200)
		}
		return "ok"
	}
	if matches := failTestRe.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m[1])
		}
		return fmt.Sprintf("FAIL: %d 个测试失败 (%s)", len(names), strings.Join(names, ", "))
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "error") || strings.Contains(line, ": ") {
			return truncateRunes(strings.TrimSpace(line), 200)
		}
	}
	trimmed := strings.TrimSpace(output)
	return truncateRunes(trimmed, 200)
}

// evidenceID 用内容哈希生成，避免同一秒内多次验证因时间戳前缀相同而撞 ID。
func evidenceID(kind core.EvidenceKind, target string, nano int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", kind, target, nano)))
	return hex.EncodeToString(sum[:])[:8]
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
