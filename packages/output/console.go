package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/fatih/color"
)

// formatValue formats a value for display, truncating or summarizing large values
func formatValue(v any, maxLen int) string {
	switch val := v.(type) {
	case []any:
		return fmt.Sprintf("[array with %d items]", len(val))
	case map[string]any:
		return fmt.Sprintf("{object with %d keys}", len(val))
	case map[string]string:
		return fmt.Sprintf("{map with %d entries}", len(val))
	case map[string][]string:
		return fmt.Sprintf("{headers with %d entries}", len(val))
	}
	str := fmt.Sprintf("%v", v)
	if len(str) > maxLen {
		return str[:maxLen] + "..."
	}
	return str
}

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
					fmt.Fprintf(f.writer, "      Expected: %s\n", formatValue(a.Expected, 100))
					fmt.Fprintf(f.writer, "      Actual:   %s\n", formatValue(a.Actual, 100))
					if a.Message != "" {
						fmt.Fprintf(f.writer, "      %s\n", a.Message)
					}
					// Show diff for complex objects when verbose is enabled
					if f.verbose {
						diff := f.formatDiff(a.Expected, a.Actual)
						if diff != "" {
							fmt.Fprint(f.writer, diff)
						}
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

// DiffResult represents a single difference between expected and actual values.
type DiffResult struct {
	Path     string
	Expected any
	Actual   any
	Type     DiffType
}

// DiffType represents the type of difference.
type DiffType int

const (
	DiffTypeChanged DiffType = iota
	DiffTypeAdded
	DiffTypeRemoved
)

// computeJSONDiff compares two values and returns a list of differences.
// It only returns differences, not the full structure.
func computeJSONDiff(expected, actual any, path string) []DiffResult {
	var diffs []DiffResult

	// Handle nil cases
	if expected == nil && actual == nil {
		return diffs
	}
	if expected == nil {
		return []DiffResult{{Path: path, Expected: nil, Actual: actual, Type: DiffTypeAdded}}
	}
	if actual == nil {
		return []DiffResult{{Path: path, Expected: expected, Actual: nil, Type: DiffTypeRemoved}}
	}

	// Check types
	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)

	// Type mismatch
	if expectedType != actualType {
		return []DiffResult{{Path: path, Expected: expected, Actual: actual, Type: DiffTypeChanged}}
	}

	switch e := expected.(type) {
	case map[string]any:
		a := actual.(map[string]any)
		diffs = append(diffs, compareObjects(e, a, path)...)
	case []any:
		a := actual.([]any)
		diffs = append(diffs, compareArrays(e, a, path)...)
	default:
		if !reflect.DeepEqual(expected, actual) {
			diffs = append(diffs, DiffResult{Path: path, Expected: expected, Actual: actual, Type: DiffTypeChanged})
		}
	}

	return diffs
}

func compareObjects(expected, actual map[string]any, path string) []DiffResult {
	var diffs []DiffResult

	// Collect all keys
	allKeys := make(map[string]bool)
	for k := range expected {
		allKeys[k] = true
	}
	for k := range actual {
		allKeys[k] = true
	}

	for key := range allKeys {
		keyPath := key
		if path != "" {
			keyPath = path + "." + key
		}

		expectedVal, expectedExists := expected[key]
		actualVal, actualExists := actual[key]

		if !expectedExists {
			diffs = append(diffs, DiffResult{Path: keyPath, Expected: nil, Actual: actualVal, Type: DiffTypeAdded})
		} else if !actualExists {
			diffs = append(diffs, DiffResult{Path: keyPath, Expected: expectedVal, Actual: nil, Type: DiffTypeRemoved})
		} else {
			diffs = append(diffs, computeJSONDiff(expectedVal, actualVal, keyPath)...)
		}
	}

	return diffs
}

func compareArrays(expected, actual []any, path string) []DiffResult {
	var diffs []DiffResult

	maxLen := len(expected)
	if len(actual) > maxLen {
		maxLen = len(actual)
	}

	for i := 0; i < maxLen; i++ {
		indexPath := fmt.Sprintf("%s[%d]", path, i)

		if i >= len(expected) {
			diffs = append(diffs, DiffResult{Path: indexPath, Expected: nil, Actual: actual[i], Type: DiffTypeAdded})
		} else if i >= len(actual) {
			diffs = append(diffs, DiffResult{Path: indexPath, Expected: expected[i], Actual: nil, Type: DiffTypeRemoved})
		} else {
			diffs = append(diffs, computeJSONDiff(expected[i], actual[i], indexPath)...)
		}
	}

	return diffs
}

// formatDiff formats the diff output for console display.
func (f *ConsoleFormatter) formatDiff(expected, actual any) string {
	// Try to parse as JSON for structured diff
	expectedJSON := parseToJSON(expected)
	actualJSON := parseToJSON(actual)

	if expectedJSON != nil && actualJSON != nil {
		diffs := computeJSONDiff(expectedJSON, actualJSON, "")
		if len(diffs) == 0 {
			return ""
		}

		// Sort diffs by path for consistent output
		sort.Slice(diffs, func(i, j int) bool {
			return diffs[i].Path < diffs[j].Path
		})

		red := color.New(color.FgRed).SprintFunc()
		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()

		var sb strings.Builder
		sb.WriteString("      Diff:\n")

		// Limit diff output to avoid overwhelming the console
		maxDiffs := 10
		for i, diff := range diffs {
			if i >= maxDiffs {
				sb.WriteString(fmt.Sprintf("        ... and %d more differences\n", len(diffs)-maxDiffs))
				break
			}

			path := diff.Path
			if path == "" {
				path = "(root)"
			}

			switch diff.Type {
			case DiffTypeAdded:
				sb.WriteString(fmt.Sprintf("        %s %s: %s\n", green("+"), path, formatValue(diff.Actual, 60)))
			case DiffTypeRemoved:
				sb.WriteString(fmt.Sprintf("        %s %s: %s\n", red("-"), path, formatValue(diff.Expected, 60)))
			case DiffTypeChanged:
				sb.WriteString(fmt.Sprintf("        %s %s:\n", yellow("~"), path))
				sb.WriteString(fmt.Sprintf("          %s %s\n", red("-"), formatValue(diff.Expected, 60)))
				sb.WriteString(fmt.Sprintf("          %s %s\n", green("+"), formatValue(diff.Actual, 60)))
			}
		}

		return sb.String()
	}

	return ""
}

// parseToJSON attempts to parse a value as JSON.
func parseToJSON(v any) any {
	if v == nil {
		return nil
	}

	// Already a map or slice
	switch val := v.(type) {
	case map[string]any:
		return val
	case []any:
		return val
	case string:
		// Try to parse string as JSON
		var result any
		if err := json.Unmarshal([]byte(val), &result); err == nil {
			return result
		}
	}

	return nil
}
