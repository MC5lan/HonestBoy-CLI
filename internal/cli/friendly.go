package cli

import (
	"errors"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/errs"
	"github.com/MC5lan/HonestBoy-CLI/internal/llm"
)

// FriendlyError 把 CLI 错误转成普通人能看懂的话（不带技术前缀）。
func FriendlyError(err error) string {
	if err == nil {
		return ""
	}
	// 模型/网络错误给出可执行的下一步，而不是只留服务商原文。
	var re *llm.RetryableError
	if errors.As(err, &re) {
		if advice := statusAdvice(re.Status); advice != "" {
			return advice
		}
	}
	switch {
	case errors.Is(err, ErrEnvironment):
		msg := strings.TrimPrefix(err.Error(), "environment error: ")
		return "还没准备好: " + msg
	case errors.Is(err, ErrUsage):
		msg := strings.TrimPrefix(err.Error(), "usage error: ")
		return "参数不对: " + msg + "\n输入 hb --help 看看有哪些命令。"
	case errors.Is(err, ErrChainBroken):
		return "证据记录被改动过，校验没通过。用 hb verify 查看详情。"
	case errors.Is(err, ErrTimeout):
		return "操作超时了。可以加 --timeout 10m 再试。"
	case errors.Is(err, ErrInterrupted):
		return "已中断。"
	}
	var te *TerminalError
	if errors.As(err, &te) {
		names := map[string]string{
			"untested": "没验证就停下了（这是合法的诚实结果）",
			"denied":   "申请完成被拦下了，因为缺验证证据",
			"halt":     "主动暂停了",
			"failed":   "执行失败",
		}
		if name, ok := names[te.Terminal.String()]; ok {
			return name
		}
	}
	// 去掉内部前缀。
	msg := err.Error()
	for _, prefix := range []string{"environment error: ", "usage error: "} {
		msg = strings.TrimPrefix(msg, prefix)
	}
	return msg
}

// statusAdvice 把模型 HTTP 状态码映射成可执行的下一步。
func statusAdvice(status int) string {
	switch {
	case status == 401 || status == 403:
		return "密钥不对或没生效。检查 api_key_env、环境变量与端点；运行 hb doctor。"
	case status == 404:
		return "端点或模型不存在。检查 model 与 base_url（包括 /v1 路径）；运行 hb doctor。"
	case status == 400:
		return "请求被模型端点拒绝。检查 model、协议与 base_url；运行 hb doctor。"
	case status == 429:
		return "请求太频繁或额度不足。稍等再试，或换模型/检查额度；运行 hb doctor 查看配置。"
	case status >= 500:
		return "模型服务端错误。稍后重试；持续失败时检查 base_url 与代理。"
	case status < 400:
		if advice, ok := errs.Explain(string(errs.NetUnavailable)); ok {
			return advice
		}
	}
	return ""
}
