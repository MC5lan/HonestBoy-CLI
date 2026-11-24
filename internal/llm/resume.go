package llm

// ResumePrompt 生成断流续写消息。partial 是已经吐出的文本，带上它模型才知道从哪里继续。
func ResumePrompt(partial string) Message {
	content := "（上文被网络中断，请从截断处继续，不要重复已输出内容）"
	if partial != "" {
		const maxPartial = 4000
		if r := []rune(partial); len(r) > maxPartial {
			partial = string(r[:maxPartial]) + "…"
		}
		content += "\n已输出的内容：\n" + partial
	}
	return Message{Role: "user", Content: content}
}
