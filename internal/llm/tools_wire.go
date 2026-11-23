package llm

import "encoding/json"

// ToWireTools 把工具声明转换为 Chat Completions 的 function 格式。
// Schema 由工具注册表生成，不在协议层手写。
func ToWireTools(specs []ToolSpec) []map[string]any {
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		var schema any = json.RawMessage(`{"type":"object"}`)
		if len(s.Schema) > 0 {
			schema = s.Schema
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        s.Name,
				"description": s.Description,
				"parameters":  schema,
			},
		})
	}
	return out
}
