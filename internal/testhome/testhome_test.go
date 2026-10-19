package testhome

import (
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(Run(m))
}

func TestRunIsolatesHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home, "hitspec-test-home-") {
		t.Fatalf("UserHomeDir() = %q, want the temporary test home", home)
	}
	if got := os.Getenv("USERPROFILE"); got != home {
		t.Fatalf("USERPROFILE = %q, want %q", got, home)
	}
}
