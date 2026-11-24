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
	// 网络/连接失败必须给出 HB-NET-001 的处置建议，而不是裸 dial 错误。
	var re *llm.RetryableError
	if errors.As(err, &re) && re.Status < 400 {
		if advice, ok := errs.Explain(string(errs.NetUnavailable)); ok {
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
