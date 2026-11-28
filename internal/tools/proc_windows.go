//go:build windows

package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// setProcessGroup 在 Windows 上由 taskkill /T 负责整树清理。
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessTree 用 taskkill 清理子进程树，再杀主进程。
// 生产版本可换成 Job Object（JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE）。
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = exec.CommandContext(context.Background(), "taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}
