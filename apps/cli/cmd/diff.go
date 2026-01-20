package cmd

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	diffOutputFlag    string
	diffThresholdFlag string
)

var diffCmd = &cobra.Command{
	Use:   "diff <results1.json> <results2.json>",
	Short: "Compare two test result files",
	Long: `Compare two JSON test result files and show the differences.

This command helps identify performance regressions or improvements
between test runs.

Examples:
  hitspec diff results1.json results2.json
  hitspec diff results1.json results2.json --output html
  hitspec diff results1.json results2.json --threshold 10%`,
	Args: cobra.ExactArgs(2),
	RunE: diffCommand,
}

func init() {
	diffCmd.Flags().StringVarP(&diffOutputFlag, "output", "o", "console", "Output format: console, json, html")
	diffCmd.Flags().StringVar(&diffThresholdFlag, "threshold", "", "Fail if any test is slower by this percentage (e.g., 10%)")
}

// DiffJSONOutput represents the JSON structure of test results for comparison
type DiffJSONOutput struct {
	Summary  DiffJSONSummary `json:"summary"`
	Tests    []DiffJSONTest  `json:"tests"`
	Duration float64         `json:"duration"`
}

type DiffJSONSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type DiffJSONTest struct {
	Name     string  `json:"name"`
	File     string  `json:"file"`
	Passed   bool    `json:"passed"`
	Skipped  bool    `json:"skipped,omitempty"`
	Duration float64 `json:"duration"`
}

// DiffResult holds the comparison result
type DiffResult struct {
	File1       string
	File2       string
	Comparisons []TestComparison
	Summary     DiffSummary
}

// TestComparison represents a comparison between two test results
type TestComparison struct {
	TestName        string
	File            string
	StatusChange    string  // "improved", "regressed", "unchanged", "new", "removed"
	Duration1       float64 // ms
	Duration2       float64 // ms
	DurationChange  float64 // percentage change
	Passed1         bool
	Passed2         bool
	InFile1         bool
	InFile2         bool
}

// DiffSummary provides overall statistics
type DiffSummary struct {
	TotalTests       int
	Improved         int
	Regressed        int
	Unchanged        int
	NewTests         int
	RemovedTests     int
	AvgDuration1     float64
	AvgDuration2     float64
	TotalDuration1   float64
	TotalDuration2   float64
	ThresholdPassed  bool
	ThresholdPercent float64
}

func diffCommand(cmd *cobra.Command, args []string) error {
	file1, file2 := args[0], args[1]

	// Load both result files
	results1, err := loadResultsFile(file1)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", file1, err)
	}

	results2, err := loadResultsFile(file2)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", file2, err)
	}

	// Parse threshold if provided
	var threshold float64
	if diffThresholdFlag != "" {
		threshold, err = parseThreshold(diffThresholdFlag)
		if err != nil {
			return err
		}
	}

	// Compare results
	diff := compareResults(file1, file2, results1, results2, threshold)

	// Output in requested format
	switch strings.ToLower(diffOutputFlag) {
	case "json":
		return outputDiffJSON(diff)
	case "html":
		return outputDiffHTML(diff)
	default:
		return outputDiffConsole(diff)
	}
}

func loadResultsFile(path string) (*DiffJSONOutput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var results DiffJSONOutput
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return &results, nil
}

func parseThreshold(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid threshold %q: %w", s, err)
	}
	return v, nil
}

func compareResults(file1, file2 string, results1, results2 *DiffJSONOutput, threshold float64) *DiffResult {
	diff := &DiffResult{
		File1: file1,
		File2: file2,
		Summary: DiffSummary{
			TotalDuration1:   results1.Duration,
			TotalDuration2:   results2.Duration,
			ThresholdPercent: threshold,
			ThresholdPassed:  true,
		},
	}

	// Create maps for quick lookup
	tests1 := make(map[string]DiffJSONTest)
	tests2 := make(map[string]DiffJSONTest)

	for _, t := range results1.Tests {
		key := t.File + "::" + t.Name
		tests1[key] = t
	}
	for _, t := range results2.Tests {
		key := t.File + "::" + t.Name
		tests2[key] = t
	}

	// Track all unique test names
	allTests := make(map[string]bool)
	for key := range tests1 {
		allTests[key] = true
	}
	for key := range tests2 {
		allTests[key] = true
	}

	var totalDur1, totalDur2 float64
	var count1, count2 int

	for key := range allTests {
		t1, in1 := tests1[key]
		t2, in2 := tests2[key]

		comp := TestComparison{
			TestName: getTestNameFromKey(key),
			File:     getFileFromKey(key),
			InFile1:  in1,
			InFile2:  in2,
		}

		if in1 {
			comp.Duration1 = t1.Duration
			comp.Passed1 = t1.Passed
			totalDur1 += t1.Duration
			count1++
		}
		if in2 {
			comp.Duration2 = t2.Duration
			comp.Passed2 = t2.Passed
			totalDur2 += t2.Duration
			count2++
		}

		// Determine status change and duration change
		if in1 && in2 {
			// Calculate percentage change
			if comp.Duration1 > 0 {
				comp.DurationChange = ((comp.Duration2 - comp.Duration1) / comp.Duration1) * 100
			}

			// Determine status
			if comp.Passed1 != comp.Passed2 {
				if comp.Passed2 {
					comp.StatusChange = "improved"
					diff.Summary.Improved++
				} else {
					comp.StatusChange = "regressed"
					diff.Summary.Regressed++
				}
			} else if comp.DurationChange < -10 {
				comp.StatusChange = "improved"
				diff.Summary.Improved++
			} else if comp.DurationChange > 10 {
				comp.StatusChange = "regressed"
				diff.Summary.Regressed++
			} else {
				comp.StatusChange = "unchanged"
				diff.Summary.Unchanged++
			}

			// Check threshold
			if threshold > 0 && comp.DurationChange > threshold {
				diff.Summary.ThresholdPassed = false
			}
		} else if in1 && !in2 {
			comp.StatusChange = "removed"
			diff.Summary.RemovedTests++
		} else {
			comp.StatusChange = "new"
			diff.Summary.NewTests++
		}

		diff.Comparisons = append(diff.Comparisons, comp)
		diff.Summary.TotalTests++
	}

	if count1 > 0 {
		diff.Summary.AvgDuration1 = totalDur1 / float64(count1)
	}
	if count2 > 0 {
		diff.Summary.AvgDuration2 = totalDur2 / float64(count2)
	}

	return diff
}

func getTestNameFromKey(key string) string {
	parts := strings.SplitN(key, "::", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return key
}

func getFileFromKey(key string) string {
	parts := strings.SplitN(key, "::", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func outputDiffConsole(diff *DiffResult) error {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	fmt.Printf("\n%s\n", bold("Test Results Comparison"))
	fmt.Printf("  %s: %s\n", cyan("File 1"), diff.File1)
	fmt.Printf("  %s: %s\n\n", cyan("File 2"), diff.File2)

	// Summary
	fmt.Printf("%s\n", bold("Summary"))
	fmt.Printf("  Total Tests:    %d\n", diff.Summary.TotalTests)
	if diff.Summary.Improved > 0 {
		fmt.Printf("  Improved:       %s\n", green(fmt.Sprintf("%d", diff.Summary.Improved)))
	}
	if diff.Summary.Regressed > 0 {
		fmt.Printf("  Regressed:      %s\n", red(fmt.Sprintf("%d", diff.Summary.Regressed)))
	}
	if diff.Summary.Unchanged > 0 {
		fmt.Printf("  Unchanged:      %d\n", diff.Summary.Unchanged)
	}
	if diff.Summary.NewTests > 0 {
		fmt.Printf("  New Tests:      %s\n", cyan(fmt.Sprintf("%d", diff.Summary.NewTests)))
	}
	if diff.Summary.RemovedTests > 0 {
		fmt.Printf("  Removed Tests:  %s\n", yellow(fmt.Sprintf("%d", diff.Summary.RemovedTests)))
	}
	fmt.Println()

	// Duration comparison
	fmt.Printf("%s\n", bold("Duration"))
	fmt.Printf("  Total (File 1): %.0fms\n", diff.Summary.TotalDuration1)
	fmt.Printf("  Total (File 2): %.0fms\n", diff.Summary.TotalDuration2)
	fmt.Printf("  Avg (File 1):   %.0fms\n", diff.Summary.AvgDuration1)
	fmt.Printf("  Avg (File 2):   %.0fms\n", diff.Summary.AvgDuration2)
	fmt.Println()

	// Detailed changes
	fmt.Printf("%s\n", bold("Test Details"))
	for _, comp := range diff.Comparisons {
		var statusSymbol string
		var statusColor func(a ...interface{}) string

		switch comp.StatusChange {
		case "improved":
			statusSymbol = "↑"
			statusColor = green
		case "regressed":
			statusSymbol = "↓"
			statusColor = red
		case "new":
			statusSymbol = "+"
			statusColor = cyan
		case "removed":
			statusSymbol = "-"
			statusColor = yellow
		default:
			statusSymbol = "="
			statusColor = func(a ...interface{}) string { return fmt.Sprint(a...) }
		}

		name := comp.TestName
		if name == "" {
			name = "(unnamed)"
		}

		if comp.InFile1 && comp.InFile2 {
			changeStr := ""
			if comp.DurationChange > 0 {
				changeStr = fmt.Sprintf("+%.1f%%", comp.DurationChange)
			} else if comp.DurationChange < 0 {
				changeStr = fmt.Sprintf("%.1f%%", comp.DurationChange)
			}
			fmt.Printf("  %s %s  %.0fms → %.0fms %s\n",
				statusColor(statusSymbol),
				name,
				comp.Duration1,
				comp.Duration2,
				statusColor(changeStr))
		} else if comp.InFile1 {
			fmt.Printf("  %s %s  (removed)\n", statusColor(statusSymbol), name)
		} else {
			fmt.Printf("  %s %s  (new, %.0fms)\n", statusColor(statusSymbol), name, comp.Duration2)
		}
	}
	fmt.Println()

	// Threshold check
	if diff.Summary.ThresholdPercent > 0 {
		if diff.Summary.ThresholdPassed {
			fmt.Printf("%s Threshold check passed (max regression: %.1f%%)\n", green("✓"), diff.Summary.ThresholdPercent)
		} else {
			fmt.Printf("%s Threshold check failed (some tests exceeded %.1f%% regression)\n", red("✗"), diff.Summary.ThresholdPercent)
			return fmt.Errorf("threshold exceeded")
		}
	}

	return nil
}

func outputDiffJSON(diff *DiffResult) error {
	type JSONComparison struct {
		TestName       string  `json:"testName"`
		File           string  `json:"file,omitempty"`
		StatusChange   string  `json:"statusChange"`
		Duration1      float64 `json:"duration1,omitempty"`
		Duration2      float64 `json:"duration2,omitempty"`
		DurationChange float64 `json:"durationChange,omitempty"`
	}

	type JSONDiff struct {
		File1       string           `json:"file1"`
		File2       string           `json:"file2"`
		Summary     DiffSummary      `json:"summary"`
		Comparisons []JSONComparison `json:"comparisons"`
	}

	output := JSONDiff{
		File1:   diff.File1,
		File2:   diff.File2,
		Summary: diff.Summary,
	}

	for _, c := range diff.Comparisons {
		output.Comparisons = append(output.Comparisons, JSONComparison{
			TestName:       c.TestName,
			File:           c.File,
			StatusChange:   c.StatusChange,
			Duration1:      c.Duration1,
			Duration2:      c.Duration2,
			DurationChange: c.DurationChange,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputDiffHTML(diff *DiffResult) error {
	const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>hitspec Diff Report</title>
    <style>
        :root {
            --bg-primary: #1a1a2e;
            --bg-secondary: #16213e;
            --text-primary: #eee;
            --text-secondary: #aaa;
            --success: #00d26a;
            --error: #ff4757;
            --warning: #ffa502;
            --info: #54a0ff;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            margin: 0;
            padding: 2rem;
        }
        .container { max-width: 1000px; margin: 0 auto; }
        h1 { margin-bottom: 0.5rem; }
        .files { color: var(--text-secondary); margin-bottom: 2rem; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
        .card { background: var(--bg-secondary); padding: 1rem; border-radius: 8px; text-align: center; }
        .card .value { font-size: 1.5rem; font-weight: bold; }
        .card.improved .value { color: var(--success); }
        .card.regressed .value { color: var(--error); }
        .card.new .value { color: var(--info); }
        .card.removed .value { color: var(--warning); }
        table { width: 100%; border-collapse: collapse; background: var(--bg-secondary); border-radius: 8px; overflow: hidden; }
        th, td { padding: 0.75rem 1rem; text-align: left; }
        th { background: #0f3460; }
        tr:not(:last-child) { border-bottom: 1px solid #2d3748; }
        .improved { color: var(--success); }
        .regressed { color: var(--error); }
        .new { color: var(--info); }
        .removed { color: var(--warning); }
        .threshold { margin-top: 2rem; padding: 1rem; border-radius: 8px; }
        .threshold.passed { background: rgba(0, 210, 106, 0.1); border: 1px solid var(--success); }
        .threshold.failed { background: rgba(255, 71, 87, 0.1); border: 1px solid var(--error); }
    </style>
</head>
<body>
    <div class="container">
        <h1>hitspec Diff Report</h1>
        <div class="files">
            <div>File 1: {{.File1}}</div>
            <div>File 2: {{.File2}}</div>
        </div>
        <div class="summary">
            <div class="card"><div class="value">{{.Summary.TotalTests}}</div><div>Total</div></div>
            <div class="card improved"><div class="value">{{.Summary.Improved}}</div><div>Improved</div></div>
            <div class="card regressed"><div class="value">{{.Summary.Regressed}}</div><div>Regressed</div></div>
            <div class="card"><div class="value">{{.Summary.Unchanged}}</div><div>Unchanged</div></div>
            <div class="card new"><div class="value">{{.Summary.NewTests}}</div><div>New</div></div>
            <div class="card removed"><div class="value">{{.Summary.RemovedTests}}</div><div>Removed</div></div>
        </div>
        <table>
            <thead>
                <tr><th>Test</th><th>Status</th><th>Duration (ms)</th><th>Change</th></tr>
            </thead>
            <tbody>
                {{range .Comparisons}}
                <tr>
                    <td>{{if .TestName}}{{.TestName}}{{else}}(unnamed){{end}}</td>
                    <td class="{{.StatusChange}}">{{.StatusChange}}</td>
                    <td>{{if and .InFile1 .InFile2}}{{printf "%.0f" .Duration1}} → {{printf "%.0f" .Duration2}}{{else if .InFile1}}{{printf "%.0f" .Duration1}} (removed){{else}}{{printf "%.0f" .Duration2}} (new){{end}}</td>
                    <td class="{{.StatusChange}}">{{if and .InFile1 .InFile2}}{{if gt .DurationChange 0}}+{{end}}{{printf "%.1f" .DurationChange}}%{{end}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{if gt .Summary.ThresholdPercent 0}}
        <div class="threshold {{if .Summary.ThresholdPassed}}passed{{else}}failed{{end}}">
            {{if .Summary.ThresholdPassed}}✓ Threshold check passed (max: {{printf "%.1f" .Summary.ThresholdPercent}}%){{else}}✗ Threshold check failed (max: {{printf "%.1f" .Summary.ThresholdPercent}}%){{end}}
        </div>
        {{end}}
    </div>
</body>
</html>`

	tmpl, err := template.New("diff").Parse(htmlTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(os.Stdout, diff)
}
