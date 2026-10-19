package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testhome"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resolved(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDiscoverWalksUp(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, filepath.Join(root, "home"))
	write(t, filepath.Join(root, "proj", "hitspec.yaml"), "timeout: 1234\n")
	sub := filepath.Join(root, "proj", "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Timeout != 1234 {
		t.Fatalf("Timeout = %d, want 1234 (config in a parent directory)", got.Config.Timeout)
	}
	if want := resolved(t, filepath.Join(root, "proj", "hitspec.yaml")); got.Path != want {
		t.Fatalf("Path = %q, want %q", got.Path, want)
	}
}

func TestDiscoverNearestWinsAndYml(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, filepath.Join(root, "home"))
	write(t, filepath.Join(root, "hitspec.yaml"), "timeout: 1\n")
	write(t, filepath.Join(root, "a", "hitspec.yml"), "timeout: 2\n")
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(filepath.Join(root, "a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Timeout != 2 {
		t.Fatalf("Timeout = %d, want 2 (nearest config, .yml accepted)", got.Config.Timeout)
	}
}

func TestDiscoverStopsAtGit(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, filepath.Join(root, "home"))
	write(t, filepath.Join(root, "hitspec.yaml"), "timeout: 99\n") // outside the repo
	write(t, filepath.Join(root, "repo", ".git", "HEAD"), "ref\n")
	sub := filepath.Join(root, "repo", "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if p := FindConfigPathUp(sub); p != "" {
		t.Fatalf("found %q above the repository root", p)
	}
	// A config in the directory that holds .git is still used.
	write(t, filepath.Join(root, "repo", "hitspec.yaml"), "timeout: 5\n")
	got, err := Discover(sub)
	if err != nil || got.Config.Timeout != 5 {
		t.Fatalf("Discover = %+v, %v; want the repo-root config", got, err)
	}
}

func TestDiscoverNeverReadsHomeOrAbove(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	testhome.Set(t, home)
	write(t, filepath.Join(root, "hitspec.yaml"), "timeout: 1\n") // above $HOME
	write(t, filepath.Join(home, "hitspec.yaml"), "timeout: 2\n") // $HOME itself
	write(t, filepath.Join(home, "work", "x", ".keep"), "")
	for _, start := range []string{home, filepath.Join(home, "work", "x")} {
		if p := FindConfigPathUp(start); p != "" {
			t.Errorf("FindConfigPathUp(%s) = %q, want none", start, p)
		}
	}
	got, err := Discover(filepath.Join(home, "work"))
	if err != nil || got.Path != "" || got.Config.Timeout == 1 || got.Config.Timeout == 2 {
		t.Fatalf("Discover under $HOME = %+v, %v; want defaults", got, err)
	}
}

func TestDiscoverWithinStopsAtRoot(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, filepath.Join(root, "home"))
	write(t, filepath.Join(root, "hitspec.yaml"), "timeout: 7\n") // parent of the workspace
	ws := filepath.Join(root, "ws")
	sub := filepath.Join(ws, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if p := FindConfigPathWithin(sub, ws); p != "" {
		t.Fatalf("DiscoverWithin escaped the workspace: %q", p)
	}
	if p := FindConfigPathUp(sub); p == "" {
		t.Fatal("unbounded discovery should find the parent config")
	}
	write(t, filepath.Join(ws, "hitspec.yaml"), "timeout: 8\n")
	got, err := DiscoverWithin(sub, ws)
	if err != nil || got.Config.Timeout != 8 {
		t.Fatalf("DiscoverWithin = %+v, %v; want the workspace config", got, err)
	}
	// A start directory outside the root finds nothing.
	if p := FindConfigPathWithin(root, ws); p != "" {
		t.Fatalf("start outside root found %q", p)
	}
}

func TestUnknownKeyWarnings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hitspec.yaml")
	write(t, path, "timeout: 5000\ntiemout: 1\nstress:\n  profiles:\n    p:\n      duraton: 1s\nenvironments:\n  dev:\n    anything: goes\nheaders:\n  X-Any: 1\n")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("unknown keys must not fail the load: %v", err)
	}
	if got.Config.Timeout != 5000 {
		t.Fatalf("Timeout = %d", got.Config.Timeout)
	}
	joined := strings.Join(got.Warnings, "\n")
	for _, want := range []string{
		`:2: unknown key "tiemout" (did you mean "timeout"?)`,
		`:6: unknown key "duraton" (did you mean "duration"?)`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings %q missing %q", joined, want)
		}
	}
	if len(got.Warnings) != 2 {
		t.Errorf("got %d warnings, want 2: %v", len(got.Warnings), got.Warnings)
	}
}

func TestUnknownKeyWithoutSuggestion(t *testing.T) {
	w := unknownKeyWarnings("c.yaml", []byte("zzzzzzzz: 1\n"))
	if len(w) != 1 || strings.Contains(w[0], "did you mean") {
		t.Fatalf("warnings = %v", w)
	}
}

// Every key the Config struct defines is known, whatever fields get added: the
// list comes from the yaml tags, so this guards the derivation itself.
func TestEveryStructKeyIsKnown(t *testing.T) {
	var sb strings.Builder
	for k := range yamlFields(reflectConfigType()) {
		sb.WriteString(k + ": null\n")
	}
	if w := unknownKeyWarnings("c.yaml", []byte(sb.String())); len(w) != 0 {
		t.Fatalf("struct keys reported unknown: %v", w)
	}
}

func TestParseErrorHasFileAndLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hitspec.yaml")
	write(t, path, "timeout: 1\nenvironments: [oops\n  bad: : :\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("want a syntax error")
	}
	if !strings.Contains(err.Error(), "hitspec.yaml:") || strings.Contains(err.Error(), "yaml: ") {
		t.Fatalf("error %q should read file:line: message", err)
	}
	write(t, path, "timeout: abc\n")
	_, err = Load(path)
	if err == nil || !strings.Contains(err.Error(), "hitspec.yaml:1: cannot unmarshal") {
		t.Fatalf("type error = %v", err)
	}
}

func TestEditDistance(t *testing.T) {
	cases := map[[2]string]int{{"timeout", "tiemout"}: 1, {"abc", "abc"}: 0, {"", "ab"}: 2, {"kitten", "sitting"}: 3}
	for in, want := range cases {
		if got := editDistance(in[0], in[1]); got != want {
			t.Errorf("editDistance(%q,%q) = %d, want %d", in[0], in[1], got, want)
		}
	}
}

func reflectConfigType() reflect.Type { return reflect.TypeOf(Config{}) }
