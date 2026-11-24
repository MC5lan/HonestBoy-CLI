package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveWorkdir 按 --cwd > config.workdir > 当前目录 解析工作区。
// 配置里的目录不存在时返回可操作错误，不静默回退到其他目录。
func ResolveWorkdir(flagValue, configValue string) (string, error) {
	if flagValue != "" {
		abs, err := filepath.Abs(flagValue)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrUsage, err)
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			return "", fmt.Errorf("%w: 工作区目录不存在: %s", ErrUsage, abs)
		}
		return abs, nil
	}
	if configValue != "" {
		abs, err := filepath.Abs(configValue)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrEnvironment, err)
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			return "", fmt.Errorf("%w: 配置的工作区不存在: %s（运行 hb config set workdir 重新设置）", ErrEnvironment, abs)
		}
		return abs, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEnvironment, err)
	}
	return wd, nil
}
