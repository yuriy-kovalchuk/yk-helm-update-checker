package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func FullVersion() string {
	return fmt.Sprintf("v%s (%s) built on %s with %s", Version, Commit, BuildDate, runtime.Version())
}
