package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets testscript re-exec this test binary as the real `hitspec`
// command, so scripts exercise the actual process: stdout, stderr, and exit
// code, not an in-process cobra call.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"hitspec": main,
	})
}

// TestScripts runs every testdata/script/*.txtar. Each script gets:
//
//	$SERVER  base URL of a local, offline test server (see testserver_test.go)
//	$HOME    an empty directory inside the script's work dir, so run history
//	         and other per-user state never touch the real home directory
//
// Run one script with: go test ./apps/cli -run TestScripts/<name>
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("testdata", "script"),
		RequireExplicitExec: true,
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// exitcode N cmd [args...] runs cmd and asserts its exact exit
			// status; stdout/stderr remain available to later assertions.
			"exitcode": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg || len(args) < 2 {
					ts.Fatalf("usage: exitcode N command [args...]")
				}
				want, err := strconv.Atoi(args[0])
				if err != nil {
					ts.Fatalf("exitcode: invalid code %q", args[0])
				}
				got := 0
				if err := ts.Exec(args[1], args[2:]...); err != nil {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) {
						ts.Fatalf("exitcode: %v", err)
					}
					got = exitErr.ExitCode()
				}
				if got != want {
					ts.Fatalf("exit code = %d, want %d", got, want)
				}
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
