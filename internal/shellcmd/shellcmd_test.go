package shellcmd

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommandRunsThroughSh(t *testing.T) {
	out, err := Command(`echo "a b" | tr a-z A-Z`).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "A B" {
		t.Fatalf("output = %q, want %q", got, "A B")
	}
}

func TestScriptContextRunsAShebangScript(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	script := filepath.Join(dir, "handler.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$1\" > '"+ShellPath(marker)+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := ScriptContext(context.Background(), script, "state one").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	data, err := os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(data)) != "state one" {
		t.Fatalf("marker = %q, %v", data, err)
	}
}

func TestShellPath(t *testing.T) {
	in := filepath.Join("a", "b", "c.sh")
	want := in
	if runtime.GOOS == "windows" {
		want = "a/b/c.sh"
	}
	if got := ShellPath(in); got != want {
		t.Fatalf("ShellPath(%q) = %q, want %q", in, got, want)
	}
}
