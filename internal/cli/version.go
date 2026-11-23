package cli

import (
	"fmt"
	"runtime"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func VersionString() string {
	return fmt.Sprintf("hb %s (%s, %s, %s)", Version, Commit, Date, runtime.Version())
}
