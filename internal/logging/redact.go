package logging

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`Bearer\s+\S+`),
	regexp.MustCompile(`(?i)api[_-]?key["'\s:=]+[A-Za-z0-9_\-]{8,}`),
	regexp.MustCompile(`(?i)authorization["'\s:=]+bearer\s+\S+`),
}

// Redact 脱敏日志与事件中的密钥；对 Authorization 头一律脱敏。
func Redact(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllStringFunc(s, func(match string) string {
			switch {
			case strings.HasPrefix(strings.ToLower(match), "authorization"):
				return "Authorization: Bearer sk-***"
			case strings.Contains(strings.ToLower(match), "api_key") || strings.Contains(strings.ToLower(match), "api-key"):
				idx := strings.IndexAny(match, ":=")
				if idx >= 0 {
					return match[:idx+1] + " sk-***"
				}
				return "api_key=sk-***"
			default:
				return "sk-***"
			}
		})
	}
	return s
}
