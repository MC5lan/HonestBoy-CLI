package core

import "regexp"

var highRiskCommandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-rf?\s+/`),
	regexp.MustCompile(`(?i)\bdel\s+/f\s+/s`),
	regexp.MustCompile(`(?i)\bformat\s+[a-z]:`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`),
	regexp.MustCompile(`(?i)\bgit\s+push\s+--force\b`),
	regexp.MustCompile(`(?i)\bdrop\s+table\b`),
	regexp.MustCompile(`(?i)\bshutdown\b`),
	regexp.MustCompile(`(?i)\breg\s+delete\b`),
}

// IsHighRiskCommand 判断命令是否命中高危清单（永不自动批准）。
func IsHighRiskCommand(command string) bool {
	for _, re := range highRiskCommandPatterns {
		if re.MatchString(command) {
			return true
		}
	}
	return false
}
