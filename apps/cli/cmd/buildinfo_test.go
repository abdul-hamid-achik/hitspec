package cmd

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	vcs := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef0123"},
		{Key: "vcs.time", Value: "2026-10-19T10:00:00Z"},
		{Key: "vcs.modified", Value: "true"},
	}
	tests := []struct {
		name          string
		v, bt         string
		info          *debug.BuildInfo
		wantV, wantBT string
	}{
		{"ldflags win", "2.20.0", "2026-10-19", &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, "2.20.0", "2026-10-19"},
		{"go install module version", "dev", "unknown", &debug.BuildInfo{Main: debug.Module{Version: "v2.20.0"}}, "2.20.0", "unknown"},
		{"local build uses vcs", "dev", "unknown", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs}, "dev+0123456789ab.dirty", "2026-10-19T10:00:00Z"},
		{"untagged pseudo-version is a dev build", "dev", "unknown", &debug.BuildInfo{Main: debug.Module{Version: "v2.0.0-20261019135500-0123456789ab+dirty"}, Settings: vcs}, "dev+0123456789ab.dirty", "2026-10-19T10:00:00Z"},
		{"pseudo-version after a tag is kept", "dev", "unknown", &debug.BuildInfo{Main: debug.Module{Version: "v2.22.1-0.20261019135500-0123456789ab"}}, "2.22.1-0.20261019135500-0123456789ab", "unknown"},
		{"no build info", "dev", "unknown", nil, "dev", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, bt := resolveVersion(tt.v, tt.bt, tt.info)
			if v != tt.wantV || bt != tt.wantBT {
				t.Fatalf("resolveVersion() = (%q, %q), want (%q, %q)", v, bt, tt.wantV, tt.wantBT)
			}
		})
	}
}
