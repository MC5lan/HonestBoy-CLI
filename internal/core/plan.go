package core

import (
	"errors"
	"strconv"
)

// PlanItem 是测试计划中的一条可执行断言（文档 §4.3）。
type PlanItem struct {
	ID       string
	Check    string // 可执行的命令或明确断言
	Expect   string
	Status   string // passed | failed | untested
	Evidence *Evidence
}

// Finalize 在报告前强制三态化（文档 §4.3.3）。
// 禁止留空；passed 必须有可接地的证据，否则报错。
func Finalize(items []PlanItem) error {
	for i := range items {
		if items[i].Status == "" {
			items[i].Status = "untested"
		}
		if items[i].Status == "passed" && (items[i].Evidence == nil || !items[i].Evidence.CanGround()) {
			return errors.New("plan item " + items[i].ID + " claims passed without evidence")
		}
		if items[i].Status != "passed" && items[i].Status != "failed" && items[i].Status != "untested" {
			return errors.New("plan item " + items[i].ID + " has invalid status: " + items[i].Status)
		}
	}
	return nil
}

// RawOutput 保留原始输出前 N 字节，超出部分由系统添加截断标记（文档 §3.2.4）。
// 截断由系统做，模型无法决定保留哪部分、无法移除标记。
func RawOutput(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n[truncated by system, " + strconv.Itoa(len(s)-max) + " bytes omitted]"
}

// ParseTestPlan 解析 [TEST PLAN] 块（文档 §4.3.2）。
func ParseTestPlan(text string) []PlanItem {
	var out []PlanItem
	i := 0
	for _, line := range splitLines(text) {
		line = trimSpace(line)
		if !hasPrefix(line, "- ") {
			continue
		}
		i++
		out = append(out, PlanItem{ID: "A" + strconv.Itoa(i), Check: trimSpace(line[2:])})
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
