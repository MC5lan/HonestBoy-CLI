package llm

// ResumePrompt 生成断流续写消息；T208 实现完整策略。
func ResumePrompt(partial string) Message {
	return Message{Role: "user", Content: "（上文被网络中断，请从截断处继续，不要重复已输出内容）"}
}
