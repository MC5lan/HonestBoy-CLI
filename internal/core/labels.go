package core

import "encoding/json"

func jsonUnmarshalLocal(data []byte, v any) error { return json.Unmarshal(data, v) }

// Label 提供产品内统一的人类可读标签；core 不依赖任何 UI 包，供 CLI/TUI 共用。

var toolLabels = map[string]string{
	"read_file": "读取文件", "write_file": "写入文件", "edit_file": "编辑文件",
	"list_dir": "查看目录", "search": "搜索", "shell": "执行命令",
	"verify_build": "验证构建", "verify_test": "运行测试", "verify_vet": "静态检查",
	"apply_patch": "应用补丁", "request_done": "申请完成", "declare_untested": "声明未验证",
}

// ToolLabel 把内部工具名翻译成中文动作。
func ToolLabel(name string) string {
	if label, ok := toolLabels[name]; ok {
		return label
	}
	return name
}

// ToolAction 生成 Codex 风格的动作用语："Ran <命令>" / "Added <文件>" / "Called <工具>"。
func ToolAction(name, args string) string {
	switch name {
	case "shell":
		return "执行命令 " + firstArg(args, "command")
	case "write_file", "apply_patch":
		return "新增/修改文件 " + firstArg(args, "path")
	case "edit_file":
		return "编辑文件 " + firstArg(args, "path")
	case "read_file":
		return "读取文件 " + firstArg(args, "path")
	case "list_dir":
		return "查看目录 " + firstArg(args, "path")
	case "search":
		return "搜索 " + firstArg(args, "pattern")
	default:
		return ToolLabel(name)
	}
}

func firstArg(args, key string) string {
	var m map[string]any
	if jsonUnmarshalLocal([]byte(args), &m) != nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// TerminalLabel 把终态翻译成中文。
func TerminalLabel(t string) string {
	labels := map[string]string{
		"done": "完成", "untested": "未验证", "denied": "未完成", "halt": "已暂停", "failed": "失败",
	}
	if label, ok := labels[t]; ok {
		return label
	}
	return t
}

// GateRuleLabel 把 Gate 规则翻译成中文。
func GateRuleLabel(rule string) string {
	labels := map[string]string{
		"coverage": "有改动没验证", "evidence_failed": "验证没通过",
		"provenance": "证据来源不足", "evidence_kind": "缺验证类型",
	}
	if label, ok := labels[rule]; ok {
		return label
	}
	return rule
}
