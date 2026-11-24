package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/store"
)

// RunInit 实现 hb init：首次运行的三步设置向导（T811）。
// 无 TTY 时从 stdin 逐行读取，因此既能在终端交互，也能被脚本驱动。
func RunInit(opts GlobalOptions, args []string) error {
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout
	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnvironment, err)
	}

	fmt.Fprintln(out, "Honest Boy 首次设置（直接回车使用默认值）")
	fmt.Fprintln(out, strings.Repeat("─", 46))

	// 第一步：工作区。
	wd, _ := os.Getwd()
	fmt.Fprintf(out, "\n[1/3] 工作区根目录（沙箱边界）\n  当前目录: %s\n", wd)
	workdir := ask(in, out, "  工作区 ["+wd+"]: ", wd)
	if st, err := os.Stat(workdir); err != nil || !st.IsDir() {
		return fmt.Errorf("%w: 工作区目录不存在", ErrUsage)
	}
	cfg.Workdir = workdir

	// 第二步：模型协议 + 端点 + 模型名。
	fmt.Fprintln(out, "\n[2/3] 模型协议与端点")
	fmt.Fprintln(out, "  1) openai-chat   OpenAI 兼容 /chat/completions（默认）")
	fmt.Fprintln(out, "  2) anthropic     Anthropic Messages /v1/messages")
	fmt.Fprintln(out, "  3) responses     OpenAI Responses /responses")
	choice := ask(in, out, "  选择 [1]: ", "1")
	protocols := map[string]string{"1": "openai-chat", "2": "anthropic", "3": "responses", "": "openai-chat"}
	protocol, ok := protocols[strings.TrimSpace(choice)]
	if !ok {
		return ErrUsage
	}
	cfg.Protocol = protocol

	defaultBase := map[string]string{
		"openai-chat": "https://api.openai.com/v1",
		"anthropic":   "https://api.anthropic.com",
		"responses":   "https://api.openai.com/v1",
	}[protocol]
	if cfg.BaseURL != "" {
		defaultBase = cfg.BaseURL
	}
	cfg.BaseURL = ask(in, out, "  base_url ["+defaultBase+"]: ", defaultBase)
	defaultModel := map[string]string{
		"openai-chat": "gpt-5", "anthropic": "claude-sonnet-4", "responses": "gpt-5",
	}[protocol]
	if cfg.Model != "" {
		defaultModel = cfg.Model
	}
	cfg.Model = ask(in, out, "  模型 ["+defaultModel+"]: ", defaultModel)

	// 第三步：密钥环境变量名。
	fmt.Fprintln(out, "\n[3/3] 密钥（只存环境变量名，绝不写进配置文件）")
	keyEnv := ask(in, out, "  密钥环境变量名 ["+cfg.APIKeyEnv+"]: ", cfg.APIKeyEnv)
	// 防呆：用户容易把密钥值当成变量名填进来（以 sk- 开头或包含一长串随机字符）。
	for strings.HasPrefix(strings.ToLower(keyEnv), "sk-") || len(keyEnv) > 40 {
		fmt.Fprintln(out, "  ⚠ 这看起来是密钥本身，不是变量名。变量名应该像 HB_API_KEY 这样的短名字。")
		keyEnv = ask(in, out, "  请重新输入变量名 ["+cfg.APIKeyEnv+"]: ", cfg.APIKeyEnv)
	}
	cfg.APIKeyEnv = keyEnv
	current := os.Getenv(keyEnv)
	if current == "" {
		fmt.Fprintf(out, "  当前 %s 未设置。请稍后执行:\n", keyEnv)
		fmt.Fprintf(out, "    PowerShell:  $env:%s='你的密钥'\n", keyEnv)
		fmt.Fprintf(out, "    CMD:         set %s=你的密钥\n", keyEnv)
		fmt.Fprintln(out, "    Linux/macOS: export "+keyEnv+"=你的密钥")
	} else {
		fmt.Fprintf(out, "  %s 已设置（值不会显示）\n", keyEnv)
	}

	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("%w: 保存配置失败: %v", ErrEnvironment, err)
	}
	fmt.Fprintln(out, "\n"+strings.Repeat("─", 46))
	fmt.Fprintln(out, "✓ 设置已保存到 "+store.ConfigPath())
	fmt.Fprintf(out, "✓ 工作区: %s（已保存为默认工作区）\n", workdir)
	fmt.Fprintf(out, "✓ 试试: hb run \"读一下 README.md 并总结\"\n")
	return nil
}

func ask(in *bufio.Reader, out io.Writer, prompt, fallback string) string {
	fmt.Fprint(out, prompt)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return fallback
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback
	}
	return line
}
