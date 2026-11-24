package render

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	imagePattern  = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`)
)

// prepareMarkdown 在交给 glamour 前处理代码块、图片与 base64。
// 代码块默认加行号；超过 200 行且未展开时折叠为前 30 行。
func prepareMarkdown(src string, expand bool) string {
	lines := strings.SplitAfter(src, "\n")
	var out strings.Builder
	var code []string
	lang := ""
	inCode := false
	flushCode := func() {
		if !inCode {
			return
		}
		body := strings.TrimSuffix(strings.Join(code, ""), "\n")
		if body != "" {
			content := body
			if !expand {
				content, _ = FoldCode(content, 200)
			}
			fenced := "```" + lang + "\n" + withCodeLineNumbers(content) + "\n```\n"
			out.WriteString(fenced)
		}
		code = nil
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inCode {
				inCode = true
				lang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				code = nil
				continue
			}
			flushCode()
			inCode = false
			lang = ""
			continue
		}
		if inCode {
			code = append(code, line)
			continue
		}
		text := imagePattern.ReplaceAllString(line, "[图片: $1 ($2)]")
		text = base64Pattern.ReplaceAllStringFunc(text, func(s string) string {
			return fmt.Sprintf("[base64 %.1fKB]", float64(len(s))/1024)
		})
		out.WriteString(text)
	}
	flushCode()
	return out.String()
}

func withCodeLineNumbers(src string) string {
	lines := strings.Split(src, "\n")
	width := len(fmt.Sprint(len(lines)))
	for i, line := range lines {
		lines[i] = fmt.Sprintf("%*d │ %s", width, i+1, line)
	}
	return strings.Join(lines, "\n")
}
