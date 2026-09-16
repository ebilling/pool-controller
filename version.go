package main

import (
	"runtime"
	"runtime/debug"
)

// version is set at link time:
//
//	go build -ldflags "-X main.version=$(git rev-parse --short=12 HEAD)"
//
// When it is left as "unknown", Go's own VCS stamp is used if the binary was
// built from a git checkout (`go build` does this by default).
var version = "unknown"

func buildVersion() string {
	if version != "" && version != "unknown" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	rev, dirty := "", ""
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
			if len(rev) > 12 {
				rev = rev[:12]
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev == "" {
		return version
	}
	return rev + dirty
}

func versionLine() string {
	return "pool-controller " + buildVersion() + " go=" + runtime.Version()
}
