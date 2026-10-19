package cmd

import (
	"runtime/debug"
	"strings"
)

// resolveVersion fills in version and build time for binaries built without
// -ldflags (go install, go build). Release builds inject both via ldflags and
// are returned unchanged.
func resolveVersion(v, bt string, info *debug.BuildInfo) (string, string) {
	if info == nil || (v != "" && v != "dev") {
		return v, bt
	}
	if mv := info.Main.Version; mv != "" && mv != "(devel)" {
		v = strings.TrimPrefix(mv, "v")
	}
	var revision, modified, vcsTime string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		case "vcs.time":
			vcsTime = s.Value
		}
	}
	if v == "" || v == "dev" {
		v = "dev"
		if revision != "" {
			if len(revision) > 12 {
				revision = revision[:12]
			}
			v += "+" + revision
			if modified == "true" {
				v += ".dirty"
			}
		}
	}
	if (bt == "" || bt == "unknown") && vcsTime != "" {
		bt = vcsTime
	}
	return v, bt
}
