package errs

import (
	"errors"
	"fmt"
	"strings"
)

// Code 是 HB-<域>-<编号> 稳定错误码。
type Code string

const (
	NetUnavailable Code = "HB-NET-001"
	NetTimeout     Code = "HB-NET-002"
	SbxEscape      Code = "HB-SBX-003"
	ToolFailed     Code = "HB-TOOL-001"
	GateDenied     Code = "HB-GATE-002"
	CfgInvalid     Code = "HB-CFG-001"
	IOFailed       Code = "HB-IO-001"
)

// Errors 是所有已登记错误码及其中文处置建议（hb explain 读取）。
var Errors = map[Code]string{
	NetUnavailable: "模型端点无法连接。检查 base_url、HTTPS_PROXY 与防火墙；用 hb doctor 自检。",
	NetTimeout:     "请求或流式响应超时。检查网络；可用 --timeout 增大整次运行超时。",
	SbxEscape:      "路径越出工作区沙箱。使用工作区内的相对路径；如需读取外部文件请在输入中显式给出路径并批准。",
	ToolFailed:     "工具执行失败。查看 tool_end 的原文输出，按提示修正参数后重试。",
	GateDenied:     "完成申请被 Gate 拒绝。按 gate_deny 的 Missing/Reason 补充验证；无证据不能完成。",
	CfgInvalid:     "配置无效。用 hb config list 检查；密钥只能放环境变量，配置里写 api_key_env。",
	IOFailed:       "磁盘读写失败。检查路径权限与剩余空间。",
}

type Error struct {
	Code   Code
	Detail string
	Cause  error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Detail, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

func (e *Error) Unwrap() error { return e.Cause }

// New 用错误码构造错误。
func New(code Code, format string, args ...any) error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Wrap 在保留错误码的同时包装底层原因。
func Wrap(code Code, cause error, format string, args ...any) error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...), Cause: cause}
}

// CodeOf 提取错误码；没有码时返回空。
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// Explain 返回错误码的中文说明。
func Explain(code string) (string, bool) {
	s, ok := Errors[Code(strings.ToUpper(code))]
	return s, ok
}
