// Package version exposes build metadata injected at link time via -ldflags.
//
//	go build -ldflags "-X github.com/CarlosArguelloDev/okshield/internal/version.Version=v0.1.0 \
//	  -X github.com/CarlosArguelloDev/okshield/internal/version.Commit=$(git rev-parse --short HEAD)"
package version

import (
	"fmt"
	"runtime"
)

// Populated by -ldflags at build time; defaults are used for local dev builds.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a single-line, human readable build description.
func String() string {
	return fmt.Sprintf("okshield %s (commit %s, built %s, %s/%s, %s)",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
