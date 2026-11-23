package cli

import (
	"fmt"
	"os"

	"github.com/MC5lan/HonestBoy-CLI/internal/errs"
)

// RunExplain 实现 hb explain HB-NET-001。
func RunExplain(opts GlobalOptions, args []string) error {
	if len(args) < 1 {
		return ErrUsage
	}
	msg, ok := errs.Explain(args[0])
	if !ok {
		return ErrUsage
	}
	fmt.Fprintf(os.Stdout, "%s: %s\n", args[0], msg)
	return nil
}
