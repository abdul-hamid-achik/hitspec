package output

import (
	"fmt"
	"io"
	"os"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/fatih/color"
)

type ConsoleFormatter struct {
	writer  io.Writer
	verbose bool
	noColor bool
}

type ConsoleOption func(*ConsoleFormatter)

func NewConsoleFormatter(opts ...ConsoleOption) *ConsoleFormatter {
	f := &ConsoleFormatter{
		writer: os.Stdout,
	}
	for _, opt := range opts {
		opt(f)
	}
	if f.noColor {
		color.NoColor = true
	}
	return f
}

func WithWriter(w io.Writer) ConsoleOption {
	return func(f *ConsoleFormatter) {
		f.writer = w
	}
}

func WithVerbose(v bool) ConsoleOption {
	return func(f *ConsoleFormatter) {
		f.verbose = v
	}
}

func WithNoColor(nc bool) ConsoleOption {
	return func(f *ConsoleFormatter) {
		f.noColor = nc
	}
}

func (f *ConsoleFormatter) FormatResult(result *runner.RunResult) {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	fmt.Fprintf(f.writer, "\n%s\n", bold("Running: "+result.File))
	fmt.Fprintf(f.writer, "\n")

	for _, r := range result.Results {
		if r.Skipped {
			fmt.Fprintf(f.writer, "  %s %s", yellow("-"), r.Name)
			if r.SkipReason != "" && r.SkipReason != "filtered out" {
				fmt.Fprintf(f.writer, " (%s)", r.SkipReason)
			}
			fmt.Fprintf(f.writer, "\n")
			continue
		}

		if r.Error != nil {
			fmt.Fprintf(f.writer, "  %s %s %s\n", red("x"), r.Name, red(fmt.Sprintf("(%v)", r.Error)))
			continue
		}

		symbol := green("✓")
		if !r.Passed {
			symbol = red("✗")
		}

		fmt.Fprintf(f.writer, "  %s %s %s\n", symbol, r.Name, cyan(fmt.Sprintf("(%dms)", r.Duration.Milliseconds())))

		if f.verbose && r.Response != nil {
			fmt.Fprintf(f.writer, "    Status: %d\n", r.Response.StatusCode)
		}

		if !r.Passed && len(r.Assertions) > 0 {
			for _, a := range r.Assertions {
				if !a.Passed {
					fmt.Fprintf(f.writer, "    %s %s %s\n", red("→"), a.Subject, a.Operator)
					fmt.Fprintf(f.writer, "      Expected: %v\n", a.Expected)
					fmt.Fprintf(f.writer, "      Actual:   %v\n", a.Actual)
					if a.Message != "" {
						fmt.Fprintf(f.writer, "      %s\n", a.Message)
					}
				}
			}
		}

		if f.verbose && len(r.Captures) > 0 {
			fmt.Fprintf(f.writer, "    Captures:\n")
			for name, value := range r.Captures {
				fmt.Fprintf(f.writer, "      %s = %v\n", name, value)
			}
		}
	}

	fmt.Fprintf(f.writer, "\n")
	fmt.Fprintf(f.writer, "Tests: ")
	if result.Passed > 0 {
		fmt.Fprintf(f.writer, "%s, ", green(fmt.Sprintf("%d passed", result.Passed)))
	}
	if result.Failed > 0 {
		fmt.Fprintf(f.writer, "%s, ", red(fmt.Sprintf("%d failed", result.Failed)))
	}
	if result.Skipped > 0 {
		fmt.Fprintf(f.writer, "%s, ", yellow(fmt.Sprintf("%d skipped", result.Skipped)))
	}
	total := result.Passed + result.Failed + result.Skipped
	fmt.Fprintf(f.writer, "%d total\n", total)
	fmt.Fprintf(f.writer, "Time:  %dms\n", result.Duration.Milliseconds())
	fmt.Fprintf(f.writer, "\n")
}

func (f *ConsoleFormatter) FormatError(err error) {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Fprintf(f.writer, "%s %v\n", red("Error:"), err)
}

func (f *ConsoleFormatter) FormatHeader(version string) {
	bold := color.New(color.Bold).SprintFunc()
	fmt.Fprintf(f.writer, "%s %s\n", bold("hitspec"), version)
}
