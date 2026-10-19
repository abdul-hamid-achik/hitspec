package result

import (
	"strings"
	"testing"
)

func TestEnvironmentsYAMLIsSorted(t *testing.T) {
	r := &Result{
		DefaultEnvironment: "base",
		Environments: map[string]map[string]any{
			"prod": {"z": 1, "a": "x"},
			"base": {"url": "http://b"},
		},
	}
	got, err := r.EnvironmentsYAML()
	if err != nil {
		t.Fatal(err)
	}
	want := "defaultEnvironment: base\nenvironments:\n    base:\n        url: http://b\n    prod:\n        a: x\n        z: 1\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWarnDeduplicates(t *testing.T) {
	r := &Result{}
	r.Warn("a %d", 1)
	r.Warn("a %d", 1)
	r.Warn("b")
	if len(r.Warnings) != 2 {
		t.Errorf("warnings = %v", r.Warnings)
	}
}

func TestContentWithEnvironments(t *testing.T) {
	r := &Result{Content: "### x\nGET http://a\n"}
	if r.ContentWithEnvironments() != r.Content {
		t.Error("content must be untouched without environments")
	}
	r.Environments = map[string]map[string]any{"dev": {"k": "v"}}
	out := r.ContentWithEnvironments()
	if !strings.Contains(out, "# environments:") || !strings.Contains(out, "#         k: v") {
		t.Errorf("environments must be appended as comments:\n%s", out)
	}
}
