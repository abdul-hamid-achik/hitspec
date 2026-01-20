// Package coverage provides API coverage reporting for hitspec.
package coverage

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Report represents an API coverage report.
type Report struct {
	TotalEndpoints   int               `json:"totalEndpoints"`
	CoveredEndpoints int               `json:"coveredEndpoints"`
	CoveragePercent  float64           `json:"coveragePercent"`
	ByTag            map[string]*TagReport `json:"byTag,omitempty"`
	Endpoints        []EndpointStatus  `json:"endpoints"`
}

// TagReport represents coverage for a specific tag.
type TagReport struct {
	Tag              string  `json:"tag"`
	TotalEndpoints   int     `json:"totalEndpoints"`
	CoveredEndpoints int     `json:"coveredEndpoints"`
	CoveragePercent  float64 `json:"coveragePercent"`
}

// EndpointStatus represents the coverage status of an endpoint.
type EndpointStatus struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operationId,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Covered     bool     `json:"covered"`
	TestCount   int      `json:"testCount"`
}

// Analyzer analyzes API coverage against an OpenAPI spec.
type Analyzer struct {
	endpoints []Endpoint
}

// Endpoint represents an API endpoint from the OpenAPI spec.
type Endpoint struct {
	Method      string
	Path        string
	OperationID string
	Tags        []string
}

// ExecutedRequest represents a request that was executed during testing.
type ExecutedRequest struct {
	Method string
	Path   string
}

// NewAnalyzer creates a new coverage analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		endpoints: make([]Endpoint, 0),
	}
}

// LoadOpenAPI loads endpoints from an OpenAPI specification file.
func (a *Analyzer) LoadOpenAPI(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec map[string]any

	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, &spec); err != nil {
		if err := json.Unmarshal(data, &spec); err != nil {
			return fmt.Errorf("failed to parse spec as YAML or JSON: %w", err)
		}
	}

	return a.parseOpenAPISpec(spec)
}

func (a *Analyzer) parseOpenAPISpec(spec map[string]any) error {
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return fmt.Errorf("no paths found in OpenAPI spec")
	}

	for path, pathItem := range paths {
		pathObj, ok := pathItem.(map[string]any)
		if !ok {
			continue
		}

		methods := []string{"get", "post", "put", "patch", "delete", "options", "head"}
		for _, method := range methods {
			operation, ok := pathObj[method].(map[string]any)
			if !ok {
				continue
			}

			endpoint := Endpoint{
				Method: strings.ToUpper(method),
				Path:   path,
			}

			if opID, ok := operation["operationId"].(string); ok {
				endpoint.OperationID = opID
			}

			if tags, ok := operation["tags"].([]any); ok {
				for _, tag := range tags {
					if tagStr, ok := tag.(string); ok {
						endpoint.Tags = append(endpoint.Tags, tagStr)
					}
				}
			}

			a.endpoints = append(a.endpoints, endpoint)
		}
	}

	return nil
}

// Analyze compares executed requests against the OpenAPI spec.
func (a *Analyzer) Analyze(requests []ExecutedRequest) *Report {
	report := &Report{
		TotalEndpoints: len(a.endpoints),
		ByTag:          make(map[string]*TagReport),
		Endpoints:      make([]EndpointStatus, 0),
	}

	// Track which endpoints were covered and how many times
	coverageCount := make(map[string]int)

	for _, req := range requests {
		for _, endpoint := range a.endpoints {
			if a.matchEndpoint(req, endpoint) {
				key := endpoint.Method + " " + endpoint.Path
				coverageCount[key]++
				break
			}
		}
	}

	// Build endpoint statuses
	for _, endpoint := range a.endpoints {
		key := endpoint.Method + " " + endpoint.Path
		count := coverageCount[key]
		covered := count > 0

		status := EndpointStatus{
			Method:      endpoint.Method,
			Path:        endpoint.Path,
			OperationID: endpoint.OperationID,
			Tags:        endpoint.Tags,
			Covered:     covered,
			TestCount:   count,
		}
		report.Endpoints = append(report.Endpoints, status)

		if covered {
			report.CoveredEndpoints++
		}

		// Update tag reports
		for _, tag := range endpoint.Tags {
			tagReport, exists := report.ByTag[tag]
			if !exists {
				tagReport = &TagReport{Tag: tag}
				report.ByTag[tag] = tagReport
			}
			tagReport.TotalEndpoints++
			if covered {
				tagReport.CoveredEndpoints++
			}
		}
	}

	// Calculate percentages
	if report.TotalEndpoints > 0 {
		report.CoveragePercent = float64(report.CoveredEndpoints) / float64(report.TotalEndpoints) * 100
	}

	for _, tagReport := range report.ByTag {
		if tagReport.TotalEndpoints > 0 {
			tagReport.CoveragePercent = float64(tagReport.CoveredEndpoints) / float64(tagReport.TotalEndpoints) * 100
		}
	}

	// Sort endpoints by path and method
	sort.Slice(report.Endpoints, func(i, j int) bool {
		if report.Endpoints[i].Path != report.Endpoints[j].Path {
			return report.Endpoints[i].Path < report.Endpoints[j].Path
		}
		return report.Endpoints[i].Method < report.Endpoints[j].Method
	})

	return report
}

// matchEndpoint checks if an executed request matches an OpenAPI endpoint.
func (a *Analyzer) matchEndpoint(req ExecutedRequest, endpoint Endpoint) bool {
	if req.Method != endpoint.Method {
		return false
	}

	// Convert OpenAPI path parameters to regex
	// e.g., /users/{id} -> /users/[^/]+
	pathPattern := regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(endpoint.Path, `[^/]+`)
	pathPattern = "^" + pathPattern + "$"

	matched, _ := regexp.MatchString(pathPattern, req.Path)
	return matched
}

// FormatConsole formats the report for console output.
func (r *Report) FormatConsole() string {
	var sb strings.Builder

	sb.WriteString("\nAPI Coverage Report\n")
	sb.WriteString("==================\n\n")

	sb.WriteString(fmt.Sprintf("Total Endpoints:   %d\n", r.TotalEndpoints))
	sb.WriteString(fmt.Sprintf("Covered Endpoints: %d\n", r.CoveredEndpoints))
	sb.WriteString(fmt.Sprintf("Coverage:          %.1f%%\n\n", r.CoveragePercent))

	// Show by tag if any
	if len(r.ByTag) > 0 {
		sb.WriteString("Coverage by Tag:\n")

		// Sort tags
		tags := make([]string, 0, len(r.ByTag))
		for tag := range r.ByTag {
			tags = append(tags, tag)
		}
		sort.Strings(tags)

		for _, tag := range tags {
			tagReport := r.ByTag[tag]
			sb.WriteString(fmt.Sprintf("  %s: %d/%d (%.1f%%)\n",
				tag, tagReport.CoveredEndpoints, tagReport.TotalEndpoints, tagReport.CoveragePercent))
		}
		sb.WriteString("\n")
	}

	// Show endpoint details
	sb.WriteString("Endpoint Details:\n")
	for _, endpoint := range r.Endpoints {
		status := "[ ]"
		if endpoint.Covered {
			status = "[x]"
		}
		sb.WriteString(fmt.Sprintf("  %s %s %s", status, endpoint.Method, endpoint.Path))
		if endpoint.TestCount > 1 {
			sb.WriteString(fmt.Sprintf(" (x%d)", endpoint.TestCount))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatJSON formats the report as JSON.
func (r *Report) FormatJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatHTML formats the report as HTML.
func (r *Report) FormatHTML() string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
  <title>API Coverage Report</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; }
    h1 { color: #333; }
    .summary { background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0; }
    .summary h2 { margin-top: 0; }
    .coverage-bar { background: #e0e0e0; height: 24px; border-radius: 4px; overflow: hidden; }
    .coverage-fill { background: #4caf50; height: 100%; }
    table { border-collapse: collapse; width: 100%; margin: 20px 0; }
    th, td { text-align: left; padding: 12px; border-bottom: 1px solid #ddd; }
    th { background: #f5f5f5; }
    .covered { color: #4caf50; }
    .uncovered { color: #f44336; }
    .tag { display: inline-block; background: #e3f2fd; color: #1976d2; padding: 2px 8px; border-radius: 4px; margin: 2px; font-size: 0.9em; }
  </style>
</head>
<body>
`)

	sb.WriteString("<h1>API Coverage Report</h1>\n")

	// Summary
	sb.WriteString("<div class=\"summary\">\n")
	sb.WriteString("<h2>Summary</h2>\n")
	sb.WriteString(fmt.Sprintf("<p><strong>Coverage:</strong> %.1f%% (%d/%d endpoints)</p>\n",
		r.CoveragePercent, r.CoveredEndpoints, r.TotalEndpoints))
	sb.WriteString("<div class=\"coverage-bar\">\n")
	sb.WriteString(fmt.Sprintf("<div class=\"coverage-fill\" style=\"width: %.1f%%\"></div>\n", r.CoveragePercent))
	sb.WriteString("</div>\n")
	sb.WriteString("</div>\n")

	// Endpoints table
	sb.WriteString("<h2>Endpoints</h2>\n")
	sb.WriteString("<table>\n")
	sb.WriteString("<tr><th>Status</th><th>Method</th><th>Path</th><th>Tags</th><th>Tests</th></tr>\n")

	for _, endpoint := range r.Endpoints {
		statusClass := "uncovered"
		statusIcon := "&#x2717;"
		if endpoint.Covered {
			statusClass = "covered"
			statusIcon = "&#x2713;"
		}

		sb.WriteString("<tr>\n")
		sb.WriteString(fmt.Sprintf("<td class=\"%s\">%s</td>\n", statusClass, statusIcon))
		sb.WriteString(fmt.Sprintf("<td><strong>%s</strong></td>\n", endpoint.Method))
		sb.WriteString(fmt.Sprintf("<td>%s</td>\n", endpoint.Path))
		sb.WriteString("<td>")
		for _, tag := range endpoint.Tags {
			sb.WriteString(fmt.Sprintf("<span class=\"tag\">%s</span>", tag))
		}
		sb.WriteString("</td>\n")
		sb.WriteString(fmt.Sprintf("<td>%d</td>\n", endpoint.TestCount))
		sb.WriteString("</tr>\n")
	}

	sb.WriteString("</table>\n")
	sb.WriteString("</body>\n</html>")

	return sb.String()
}
