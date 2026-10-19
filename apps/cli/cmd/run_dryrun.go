package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/output"
)

// writeDryRun prints what `hitspec run` would do with file, without sending
// anything: each request in execution order with its method and URL (resolved
// from the environment and file variables; references to captured values stay
// as written), its tags and dependencies, and why a request would be skipped.
// It returns how many requests satisfy --name/--tags, and the error a real run
// would report for a file that cannot load.
func writeDryRun(w io.Writer, r *runner.Runner, file string) (matched int, err error) {
	plan, err := r.PlanFile(file)
	if err != nil {
		return 0, err
	}

	running, skipped := 0, 0
	for _, p := range plan.Requests {
		if p.SkipReason == "" {
			running++
		} else {
			skipped++
		}
	}
	fmt.Fprintf(w, "Would run: %s (%d to run, %d skipped)\n", file, running, skipped)

	n := 0
	for _, p := range plan.Requests {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("line %d", p.Line)
		}
		marker := "-"
		if p.SkipReason == "" {
			n++
			marker = fmt.Sprintf("%d.", n)
		}
		fmt.Fprintf(w, "  %-3s %s  %s %s\n", marker, name, p.Method, output.RedactURL(p.URL))
		if len(p.Tags) > 0 {
			fmt.Fprintf(w, "        tags: %s\n", strings.Join(p.Tags, ", "))
		}
		if len(p.Depends) > 0 {
			fmt.Fprintf(w, "        depends: %s\n", strings.Join(p.Depends, ", "))
		}
		if p.Pending != "" {
			fmt.Fprintf(w, "        note: %s\n", p.Pending)
		}
		if p.SkipReason != "" {
			fmt.Fprintf(w, "        skipped: %s\n", p.SkipReason)
		}
	}
	return plan.Matched, nil
}
