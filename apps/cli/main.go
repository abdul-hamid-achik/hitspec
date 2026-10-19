package main

import (
	"github.com/abdul-hamid-achik/hitspec/v2/apps/cli/cmd"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.Execute(version, buildTime)
}
