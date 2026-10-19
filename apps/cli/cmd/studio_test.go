package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testhome"
)

// TestNewStudioManager_EnvAndConfigReachManager guards the regression where
// `studio --env` and `--config` were dropped (a duplicated WithLogLevel sat
// where WithEnv/WithConfigPath belonged). `hitspec serve` without --api-only
// goes through launchStudio with the same studioLaunchFlags, so it is covered
// by the same constructor.
func TestNewStudioManager_EnvAndConfigReachManager(t *testing.T) {
	testhome.Set(t, t.TempDir()) // keep the history database out of the real ~/.hitspec

	// workDir holds a decoy hitspec.yaml; the real config lives in another
	// directory. If --config is ignored (or its load silently falls back to the
	// workDir search) the decoy wins and the assertions below fail.
	workDir := t.TempDir()
	decoy := "defaultEnvironment: decoy\nenvironments:\n  decoy:\n    marker: decoy\n"
	if err := os.WriteFile(filepath.Join(workDir, "hitspec.yaml"), []byte(decoy), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "custom.yaml")
	real := "defaultEnvironment: fromconfig\nenvironments:\n  fromconfig:\n    marker: real\n  qa:\n    marker: qa\n"
	if err := os.WriteFile(cfgPath, []byte(real), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := newStudioManager(workDir, studioLaunchFlags{
		env:       "qa",
		config:    cfgPath,
		logFormat: "text",
		logLevel:  "info",
	})
	defer mgr.Close()

	if got := mgr.Config().Env; got != "qa" {
		t.Errorf("active environment = %q, want the --env value %q", got, "qa")
	}
	if got := mgr.Config().ConfigPath; got != cfgPath {
		t.Errorf("config path = %q, want the --config value %q", got, cfgPath)
	}
	env, err := mgr.GetEnvironment(context.Background(), "qa")
	if err != nil {
		t.Fatal(err)
	}
	if env.Variables["marker"] != "qa" {
		t.Errorf("environment qa variables = %v, want the --config file's (marker=qa)", env.Variables)
	}
	if env, _ := mgr.GetEnvironment(context.Background(), "decoy"); len(env.Variables) != 0 {
		t.Errorf("workDir's decoy hitspec.yaml was loaded instead of --config: %v", env.Variables)
	}
}

// Without --env the config's defaultEnvironment (from --config) applies.
func TestNewStudioManager_DefaultEnvFromConfig(t *testing.T) {
	testhome.Set(t, t.TempDir())
	cfgPath := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(cfgPath, []byte("defaultEnvironment: fromconfig\nenvironments:\n  fromconfig: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := newStudioManager(t.TempDir(), studioLaunchFlags{config: cfgPath, logFormat: "text", logLevel: "info"})
	defer mgr.Close()
	if got := mgr.Config().Env; got != "fromconfig" {
		t.Errorf("active environment = %q, want defaultEnvironment %q from --config", got, "fromconfig")
	}
}

// While the studio runs, engine output on os.Stdout/os.Stderr/the std logger
// must not reach the terminal the UI is drawing on; afterwards everything is
// restored.
func TestSilenceStdio(t *testing.T) {
	origOut, origErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = w, w // stand-in for the real terminal
	log.SetOutput(w)
	defer func() { os.Stdout, os.Stderr = origOut, origErr; log.SetOutput(origErr) }()

	out, restore := silenceStdio()
	if out != w {
		t.Errorf("silenceStdio returned %v, want the real stdout", out)
	}
	fmt.Println("stray stdout")
	fmt.Fprintln(os.Stderr, "stray stderr")
	log.Print("stray log")
	cmd := exec.Command("echo", "from a child")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()
	restore()

	if os.Stdout != w || os.Stderr != w {
		t.Errorf("restore left os.Stdout=%v os.Stderr=%v", os.Stdout, os.Stderr)
	}
	log.Print("after restore")
	_ = w.Close()
	got, _ := io.ReadAll(r)
	if strings.Contains(string(got), "stray") || strings.Contains(string(got), "child") {
		t.Errorf("output leaked to the terminal while silenced: %q", got)
	}
	if !strings.Contains(string(got), "after restore") {
		t.Errorf("std logger not restored: %q", got)
	}
}

// TestStudioMouseCaptureIsOptIn: capturing the mouse stops the terminal from
// selecting text, and nothing in studio is clickable, so it must be off unless
// asked for.
func TestStudioMouseCaptureIsOptIn(t *testing.T) {
	f := studioCmd.Flags().Lookup("mouse")
	if f == nil {
		t.Fatal("studio has no --mouse flag")
	}
	if f.DefValue != "false" {
		t.Fatalf("--mouse defaults to %q, want false", f.DefValue)
	}
}
