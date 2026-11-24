package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlight 对代码着色；语言未知时保留行号与边框但不强行猜测。
func Highlight(src, lang, theme string) (string, error) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(src)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	styleName := "monokai"
	if theme == "light" {
		styleName = "github"
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	iterator, err := lexer.Tokenise(nil, src)
	if err != nil {
		return src, err
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return src, err
	}
	return withLineNumbers(buf.String(), strings.Count(src, "\n")+1), nil
}

func withLineNumbers(src string, lines int) string {
	width := len(fmt.Sprint(lines))
	parts := strings.Split(strings.TrimRight(src, "\n"), "\n")
	for i, line := range parts {
		parts[i] = fmt.Sprintf("%*d │ %s", width, i+1, line)
	}
	return strings.Join(parts, "\n")
}

// FoldCode 超过 200 行时默认折叠，显示前 30 行。
func FoldCode(src string, maxLines int) (string, bool) {
	lines := strings.Split(src, "\n")
	if len(lines) <= maxLines {
		return src, false
	}
	head := 30
	if maxLines < head {
		head = maxLines
	}
	return strings.Join(lines[:head], "\n") + fmt.Sprintf("\n… 折叠 %d 行（Ctrl+O 展开）", len(lines)-head), true
}
