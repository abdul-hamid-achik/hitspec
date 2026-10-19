package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// Plan is what a run of one file would do, computed without any network access
// (hitspec run --dry-run).
type Plan struct {
	File     string
	Requests []PlannedRequest // in execution order, skipped ones included
	// Matched counts the requests that satisfy --name/--tags (RunResult.Matched),
	// so the CLI can tell "the filters matched nothing" from "all were skipped".
	Matched int
}

// PlannedRequest is one request of a Plan.
type PlannedRequest struct {
	Name        string
	Line        int
	Description string
	Method      string
	// URL is the request URL with environment and file variables resolved. A
	// reference that only exists once a response was captured, such as
	// {{login.token}}, is left as written.
	URL     string
	Tags    []string
	Depends []string
	// SkipReason is why the request would not run ("filtered out", the @skip
	// text, "condition not met", "dependency failed"); empty when it would run.
	SkipReason string
	// Pending is set when the request's @if/@unless depends on a captured value,
	// so whether it runs is only known at run time.
	Pending string
}

// PlanFile parses path and reports which requests a run would execute, in
// order, with their resolved URLs. It applies the same selection as a real run
// (--name, --tags, @only, @skip, @depends, @if/@unless) but sends nothing and
// executes no hooks. Unresolved-variable warnings are muted on the runner's
// resolver: a plan is expected to have references that only a run can fill.
func (r *Runner) PlanFile(path string) (*Plan, error) {
	file, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	environment, err := env.LoadEnvironment(filepath.Dir(path), r.config.Environment, r.config.ConfigEnvironments)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment %q: %w", r.config.Environment, err)
	}
	r.resolver.SetWarnFunc(nil)
	r.resolver.SetVariables(environment.Variables)
	for _, v := range file.Variables {
		r.resolver.SetVariable(v.Name, v.Value)
	}

	hasOnly := false
	for _, req := range file.Requests {
		if req.Metadata != nil && req.Metadata.Only {
			hasOnly = true
			break
		}
	}
	sorted, err := r.topologicalSort(file.Requests)
	if err != nil {
		return nil, err
	}
	include := r.withDependencies(sorted, hasOnly)

	baseDir := filepath.Dir(file.Path)
	plan := &Plan{File: file.Path}
	willRun := map[string]bool{}
	for _, req := range sorted {
		if r.matchesFilters(req) {
			plan.Matched++
		}
		p := PlannedRequest{
			Name:        req.Name,
			Line:        req.Line,
			Description: req.Description,
			Method:      req.Method,
			Tags:        req.Tags,
		}
		if req.Metadata != nil {
			p.Depends = req.Metadata.Depends
		}
		p.URL = http.BuildRequestFromASTWithBaseDir(req, r.resolver.Resolve, baseDir).URL

		switch {
		case !include[req]:
			p.SkipReason = "filtered out"
		case req.Metadata != nil && req.Metadata.Skip != "":
			p.SkipReason = req.Metadata.Skip
		default:
			p.SkipReason, p.Pending = r.planGate(req, willRun)
		}
		if p.SkipReason == "" && req.Name != "" {
			willRun[req.Name] = true
		}
		plan.Requests = append(plan.Requests, p)
	}
	return plan, nil
}

// planGate applies the checks a run does just before sending a request that
// has not been filtered or @skip'd: its @if/@unless and its @depends. It
// returns the skip reason, or a note when the answer depends on a capture.
func (r *Runner) planGate(req *parser.Request, willRun map[string]bool) (skipReason, pending string) {
	if req.Metadata == nil {
		return "", ""
	}
	if c := req.Metadata.Condition; c != nil {
		if refsCapture(r.resolver.Resolve(c.Expression)) {
			keyword := "@if"
			if c.Type == parser.ConditionUnless {
				keyword = "@unless"
			}
			pending = fmt.Sprintf("%s %s depends on a captured value (decided at run time)", keyword, c.Expression)
		} else if !r.evaluateCondition(c) {
			return "condition not met", ""
		}
	}
	// A dependency that will not run makes its dependent skip, as in a real
	// run. A dependency that runs but fails cannot be known here.
	for _, dep := range req.Metadata.Depends {
		if !willRun[dep] {
			return "dependency will not run", ""
		}
	}
	return "", pending
}

// refsCapture reports whether text still holds a {{request.capture}} reference,
// which only exists once that request has run. A plain {{name}} left over is a
// variable that is simply undefined, which a real run treats as empty.
func refsCapture(text string) bool {
	for _, ref := range unresolvedRef.FindAllString(text, -1) {
		if strings.Contains(ref, ".") && !strings.Contains(ref, "(") {
			return true
		}
	}
	return false
}
