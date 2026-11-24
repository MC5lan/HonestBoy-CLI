package tools

import "errors"

var (
	ErrDuplicateTool     = errors.New("duplicate tool name")
	ErrInvalidToolName   = errors.New("invalid tool name")
	ErrInvalidToolSchema = errors.New("invalid tool schema")
	ErrOutsideSandbox    = errors.New("path is outside sandbox")
	ErrNeedsReadFirst    = errors.New("please read the file before writing")
	ErrChangedSinceRead  = errors.New("file changed since last read, please read again")
	ErrAmbiguousMatch    = errors.New("match is not unique")
	ErrNoMatch           = errors.New("no match found")
	ErrBinaryFile        = errors.New("binary file")
	ErrFileTooLarge      = errors.New("file too large")
	ErrLineTooLong       = errors.New("line too long")
	ErrCommandTimeout    = errors.New("command timeout: 命令超时；可给命令加 -v 输出进度，或用 timeout_ms 延长超时")
)
