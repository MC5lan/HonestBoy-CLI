// Command hb 是 Honest Boy CLI 的唯一入口，只做参数解析与装配。
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/MC5lan/HonestBoy-CLI/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:]); err != nil {
		if errors.Is(err, cli.ErrInteractive) {
			var ir *cli.InteractiveRequest
			if errors.As(err, &ir) {
				os.Exit(exitOf(runTUI(ir.ResumeID)))
			}
			os.Exit(exitOf(runTUI("")))
		}
		// 用法/环境错误给出可操作提示，而不是只留一个退出码。
		fmt.Fprintln(os.Stderr, "hb:", cli.FriendlyError(err))
		os.Exit(cli.ExitCodeOf(err))
	}
}
