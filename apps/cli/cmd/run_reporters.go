package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/spf13/pflag"
)

// reporterNames lists the formats `reporters` and -o accept, in the order the
// error messages name them.
var reporterNames = []string{"console", "json", "junit", "tap", "html"}

// reporterFiles maps a reporter to its file name inside outputDir. Console is
// not listed: it always writes to stdout.
var reporterFiles = map[string]string{
	"json":  "hitspec-results.json",
	"junit": "hitspec-junit.xml",
	"tap":   "hitspec-results.tap",
	"html":  "hitspec-report.html",
}

// reportTarget is one report destination of a run: a format and the file it
// is written to, or stdout when File is empty.
type reportTarget struct {
	Format string
	File   string
}

// reportPlan says where `run` writes its reports.
type reportPlan struct {
	Targets []reportTarget
	// Dir is the resolved outputDir to create before the run, "" when unset
	// or when no report is written there.
	Dir string
	// ListFailures is set when hitspec.yaml leaves no report on stdout, so the
	// failed requests are listed on stderr as --quiet does it.
	ListFailures bool
}

// cliPlan is the single destination of -o and --output-file.
func cliPlan(format, file string) *reportPlan {
	return &reportPlan{Targets: []reportTarget{{Format: format, File: file}}}
}

// reportFlagsGiven reports whether the report destination was chosen outside
// hitspec.yaml: -o or --output-file on the command line, or HITSPEC_OUTPUT or
// HITSPEC_OUTPUT_FILE in the environment. The environment value is baked into
// the flag's default, so Changed alone would miss it.
func reportFlagsGiven(flags *pflag.FlagSet, lookupEnv func(string) (string, bool)) bool {
	if flags != nil && (flags.Changed("output") || flags.Changed("output-file")) {
		return true
	}
	for _, name := range []string{"HITSPEC_OUTPUT", "HITSPEC_OUTPUT_FILE"} {
		if v, ok := lookupEnv(name); ok && v != "" {
			return true
		}
	}
	return false
}

// resolveReportPlan decides where `run` writes its reports. The command line
// (and its HITSPEC_* variables) wins completely; otherwise hitspec.yaml's
// `reporters` and `outputDir` apply. label names the config file in errors.
func resolveReportPlan(flags *pflag.FlagSet, lookupEnv func(string) (string, bool), fileConfig *config.Config, label string) (*reportPlan, error) {
	if reportFlagsGiven(flags, lookupEnv) {
		return cliPlan(outputFlag, outputFileFlag), nil
	}
	return planFromConfig(fileConfig, label)
}

// planFromConfig builds the plan for hitspec.yaml's `reporters` and
// `outputDir`. Names are case-insensitive, duplicates are dropped and an empty
// list means console. Without outputDir every reporter writes to stdout, so
// only one is allowed.
func planFromConfig(fileConfig *config.Config, label string) (*reportPlan, error) {
	var names []string
	seen := make(map[string]bool)
	var listed []string
	if fileConfig != nil {
		listed = fileConfig.Reporters
	}
	for _, raw := range listed {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name != "console" && reporterFiles[name] == "" {
			return nil, fmt.Errorf("%s: unknown reporter %q (valid: %s)", label, raw, strings.Join(reporterNames, ", "))
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		names = []string{"console"}
	}

	plan := &reportPlan{}
	if fileConfig != nil && fileConfig.OutputDir != "" {
		// Clean also turns the / of a path written in the yaml into \ on Windows.
		plan.Dir = filepath.Clean(fileConfig.ResolvePath(fileConfig.OutputDir))
	}
	if plan.Dir == "" && len(names) > 1 {
		return nil, fmt.Errorf("%s: reporters %s would all write to stdout; set outputDir to write the json, junit, tap and html reports to files", label, joinNames(names))
	}
	onStdout := false
	for _, name := range names {
		t := reportTarget{Format: name}
		if name != "console" && plan.Dir != "" {
			t.File = filepath.Join(plan.Dir, reporterFiles[name])
		}
		onStdout = onStdout || t.File == ""
		plan.Targets = append(plan.Targets, t)
	}
	plan.ListFailures = !onStdout
	if !onlyStdout(plan.Targets) {
		return plan, nil
	}
	// Nothing is written to outputDir, so it is not created either.
	plan.Dir = ""
	return plan, nil
}

// onlyStdout reports whether no target writes a file.
func onlyStdout(targets []reportTarget) bool {
	for _, t := range targets {
		if t.File != "" {
			return false
		}
	}
	return true
}

// joinNames spells a list as "a", "a and b" or "a, b and c".
func joinNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// ensureDir creates outputDir (and its parents); a no-op when it is unset.
func (p *reportPlan) ensureDir() error {
	if p.Dir == "" {
		return nil
	}
	return os.MkdirAll(p.Dir, 0o755)
}

// reportConfigLabel names the config file for messages about its keys: the
// --config path, else the discovered hitspec.yaml, else the default name.
func reportConfigLabel() string {
	path := configFlag
	if path == "" {
		path = config.FindConfigPathUp(".")
	}
	if path == "" {
		return config.ConfigFilenames[0]
	}
	return config.DisplayPath(path)
}

// openedReport is one target with its formatter and destination.
type openedReport struct {
	target    reportTarget
	formatter Formatter
	closeFn   func() error
}

// reportSet is the open destinations of one pass. It fans every call out to
// each target. Targets that a --quiet run would drop (stdout) are not opened.
type reportSet struct {
	reports []openedReport
}

// openReports opens a fresh destination for every target of one pass; files
// are created (truncated) so a watch re-run leaves one document per file.
func openReports(plan *reportPlan, verbose int, noColor, quiet bool) (*reportSet, error) {
	set := &reportSet{}
	for _, t := range plan.Targets {
		if quiet && t.File == "" {
			continue // --quiet drops the stdout report, but not the files
		}
		f, closeFn, err := openReport(t.Format, t.File, verbose, noColor, quiet)
		if err != nil {
			_ = set.Close()
			return nil, err
		}
		set.reports = append(set.reports, openedReport{target: t, formatter: f, closeFn: closeFn})
	}
	return set, nil
}

// Active reports whether any destination receives a report.
func (s *reportSet) Active() bool { return len(s.reports) > 0 }

func (s *reportSet) FormatHeader(version string) {
	for _, r := range s.reports {
		r.formatter.FormatHeader(version)
	}
}

func (s *reportSet) FormatResult(result *runner.RunResult) {
	for _, r := range s.reports {
		r.formatter.FormatResult(result)
	}
}

// FormatBroken reports a file that could not be run. A console report on
// stdout does not repeat the error (stderr already has it); a console report
// in a file carries it; structured formats list the file as a failed result.
func (s *reportSet) FormatBroken(file string, err error) {
	for _, r := range s.reports {
		switch {
		case r.target.Format != "console":
			r.formatter.FormatResult(brokenFileResult(file, err))
		case r.target.File != "":
			r.formatter.FormatError(err)
		}
	}
}

// Flush flushes the formatters that accumulate results and returns the first
// error.
func (s *reportSet) Flush(total time.Duration) error {
	var first error
	for _, r := range s.reports {
		if f, ok := r.formatter.(Flushable); ok {
			if err := f.Flush(total); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

// Close closes every destination and returns the first error.
func (s *reportSet) Close() error {
	var first error
	for _, r := range s.reports {
		if err := r.closeFn(); err != nil && first == nil {
			first = err
		}
	}
	s.reports = nil
	return first
}
