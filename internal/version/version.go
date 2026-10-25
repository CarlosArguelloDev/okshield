// Package version exposes build metadata injected at link time via -ldflags
// or automatically recovered from Go runtime build information.
//
//	go build -ldflags "-X github.com/CarlosArguelloDev/okshield/internal/version.Version=v0.1.0 \
//	  -X github.com/CarlosArguelloDev/okshield/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/CarlosArguelloDev/okshield/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Populated by -ldflags at build time; defaults are used for local dev builds.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		applyBuildInfo(info)
	}
}

func applyBuildInfo(info *debug.BuildInfo) {
	if info == nil {
		return
	}
	if Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
	var rev, vcsTime string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if Commit == "none" && rev != "" {
		if len(rev) > 7 {
			rev = rev[:7]
		}
		if modified {
			rev += "-dirty"
		}
		Commit = rev
	}
	if Date == "unknown" && vcsTime != "" {
		Date = vcsTime
	}
}

// String returns a single-line, human readable build description.
func String() string {
	return fmt.Sprintf("okshield %s (commit %s, built %s, %s/%s, %s)",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
