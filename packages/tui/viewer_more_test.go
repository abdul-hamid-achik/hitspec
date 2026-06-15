package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// TestStatusLineClasses covers the status-code color branches (2xx/3xx/4xx/5xx
// and the 0 "no response" case) — each must render the code and latency.
func TestStatusLineClasses(t *testing.T) {
	s := newStyles(defaultPalette())
	for _, code := range []int{200, 301, 404, 500, 0} {
		resp := &clientmgr.HTTPResponseDTO{StatusCode: code, Status: fmt.Sprintf("%d X", code), Duration: 12, Size: 100}
		out := plain(statusLine(s, resp))
		if !strings.Contains(out, fmt.Sprint(code)) {
			t.Fatalf("statusLine(%d) missing code: %q", code, out)
		}
		if !strings.Contains(out, "ms") {
			t.Fatalf("statusLine(%d) missing latency: %q", code, out)
		}
	}
}

// TestBodyTabBranches covers the body tab's error, empty-body, and
// no-requests-executed arms (the non-happy paths the happy-path test misses).
func TestBodyTabBranches(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.tab = respBody

	// A connection error (no response object) surfaces the error text.
	rv.setResult(&clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{
		{Name: "x", Error: "connection refused"},
	}})
	if got := plain(rv.tabContent()); !strings.Contains(got, "connection refused") {
		t.Fatalf("body tab should show the error, got: %q", got)
	}

	// A response with no body shows the empty-body placeholder.
	rv.setResult(&clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{
		{Name: "y", Response: &clientmgr.HTTPResponseDTO{StatusCode: 204, Status: "204 No Content"}},
	}})
	if got := plain(rv.tabContent()); !strings.Contains(got, "empty body") {
		t.Fatalf("body tab should show the empty-body placeholder, got: %q", got)
	}

	// No requests executed at all.
	rv.setResult(&clientmgr.RunResultDTO{})
	if got := plain(rv.tabContent()); !strings.Contains(got, "No requests executed") {
		t.Fatalf("body tab should handle an empty result, got: %q", got)
	}
}

// TestHintsPerFocus covers the focus-dependent and editing branches of the
// status-bar hint text.
func TestHintsPerFocus(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	cases := []struct {
		setup func()
		want  string
	}{
		{func() { m.editing = false; m.focus = focusFiles }, "n new"},
		{func() { m.focus = focusRequests }, "rows"},
		{func() { m.focus = focusResponse }, "tabs"},
		{func() { m.editing = true; m.focus = focusSource }, "ctrl+s save"},
	}
	for _, c := range cases {
		c.setup()
		if got := plain(m.hints()); !strings.Contains(got, c.want) {
			t.Fatalf("hints missing %q, got: %q", c.want, got)
		}
	}
}

func TestSmallHelpers(t *testing.T) {
	if defaultString("", "fallback") != "fallback" {
		t.Fatal("defaultString empty should return fallback")
	}
	if defaultString("v", "fallback") != "v" {
		t.Fatal("defaultString non-empty should return the value")
	}

	files := []clientmgr.FileInfoDTO{{RelativePath: "a.http"}, {RelativePath: "b.http"}}
	if !containsFile(files, "b.http") {
		t.Fatal("containsFile should find b.http")
	}
	if containsFile(files, "missing.http") {
		t.Fatal("containsFile should not find a missing file")
	}

	// lastResponseBody scans from the last request backwards for a body.
	m := newModel(context.Background(), newTestManager(t), Options{})
	if m.lastResponseBody() != "" {
		t.Fatal("lastResponseBody with no result should be empty")
	}
	m.lastResult = &clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{
		{Name: "a", Response: &clientmgr.HTTPResponseDTO{Body: "first"}},
		{Name: "b", Response: &clientmgr.HTTPResponseDTO{Body: "last"}},
	}}
	if got := m.lastResponseBody(); got != "last" {
		t.Fatalf("lastResponseBody = %q, want last", got)
	}
}
