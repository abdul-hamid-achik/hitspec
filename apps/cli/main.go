package main

import (
	"github.com/abdul-hamid-achik/hitspec/apps/cli/cmd"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.Execute(version, buildTime)
}
