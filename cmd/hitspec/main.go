// Command hitspec is the hitspec CLI. It exists so that
//
//	go install github.com/abdul-hamid-achik/hitspec/v2/cmd/hitspec@latest
//
// installs a binary named "hitspec". It is identical to ./apps/cli, which
// release builds and the Taskfile use.
package main

import "github.com/abdul-hamid-achik/hitspec/v2/apps/cli/cmd"

// Set via -ldflags "-X main.version=... -X main.buildTime=..."; otherwise
// cmd.Execute falls back to the module and VCS build info.
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.Execute(version, buildTime)
}
