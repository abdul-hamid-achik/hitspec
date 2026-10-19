package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestScriptsSignals runs the scripts that need a long-lived hitspec process
// that is sent SIGINT (--watch, Ctrl-C handling). They live apart from
// testdata/script because they need a `sleep` command, which TestScripts does
// not define (nor the `waitfile` polling command below), and cannot run on Windows (no SIGINT for child processes).
//
// Run one with: go test ./apps/cli -run TestScriptsSignals/run_watch
func TestScriptsSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts send SIGINT to the process")
	}
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("testdata", "script_signals"),
		RequireExplicitExec: true,
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// waitfile FILE REGEXP [TIMEOUT] polls until FILE exists and matches
			// REGEXP (default timeout 20s), so scripts do not depend on how long
			// a loaded machine takes to finish a pass.
			"waitfile": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg || len(args) < 2 || len(args) > 3 {
					ts.Fatalf("usage: waitfile file regexp [timeout]")
				}
				re, err := regexp.Compile(args[1])
				if err != nil {
					ts.Fatalf("waitfile: %v", err)
				}
				timeout := 20 * time.Second
				if len(args) == 3 {
					if timeout, err = time.ParseDuration(args[2]); err != nil {
						ts.Fatalf("waitfile: %v", err)
					}
				}
				path := ts.MkAbs(args[0])
				deadline := time.Now().Add(timeout)
				for {
					if data, err := os.ReadFile(path); err == nil && re.Match(data) {
						return
					}
					if time.Now().After(deadline) {
						ts.Fatalf("waitfile: %s did not match %q within %v", args[0], args[1], timeout)
					}
					time.Sleep(50 * time.Millisecond)
				}
			},
			// sleep DURATION waits, e.g. `sleep 1500ms`.
			"sleep": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg || len(args) != 1 {
					ts.Fatalf("usage: sleep duration")
				}
				d, err := time.ParseDuration(args[0])
				if err != nil {
					ts.Fatalf("sleep: %v", err)
				}
				time.Sleep(d)
			},
		},
		Setup: func(env *testscript.Env) error {
			srv := newTestServer()
			env.Defer(srv.Close)
			env.Setenv("SERVER", srv.URL)

			home := filepath.Join(env.WorkDir, ".home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				return err
			}
			env.Setenv("HOME", home)
			env.Setenv("USERPROFILE", home)
			env.Setenv("NO_COLOR", "1")
			return nil
		},
	})
}
