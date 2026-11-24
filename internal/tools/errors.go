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
	ErrCommandTimeout    = errors.New("command timeout")
)
