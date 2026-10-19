// Package shellcmd builds the *exec.Cmd values hitspec uses to run user shell
// commands (hooks, >>>shell blocks) and state-handler scripts.
//
// hitspec documents these as POSIX `sh -c`. On Unix that is simply sh. Windows
// has no sh of its own, so the shell is looked up on PATH and in the default Git
// for Windows install (which every developer using git already has); only when
// none is found does it fall back to cmd.exe.
package shellcmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Command returns a command that runs script through `sh -c`.
func Command(script string) *exec.Cmd {
	return CommandContext(context.Background(), script)
}

// CommandContext is Command bound to ctx.
func CommandContext(ctx context.Context, script string) *exec.Cmd {
	if runtime.GOOS != "windows" {
		return exec.CommandContext(ctx, "sh", "-c", script) // #nosec G204 -- running user shell commands is the feature, gated by --allow-shell.
	}
	if sh := windowsShell(); sh != "" {
		return exec.CommandContext(ctx, sh, "-c", script) // #nosec G204 -- see above.
	}
	return exec.CommandContext(ctx, "cmd", "/C", script) // #nosec G204 -- see above.
}

// ScriptContext returns a command that runs the script file at path with args.
//
// On Unix the file is executed directly (its shebang and mode decide). Windows
// cannot execute a script by its shebang, so anything that is not natively
// executable (.exe, .bat, .cmd, .com) is run through sh when sh is available.
func ScriptContext(ctx context.Context, path string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" && !nativeExecutable(path) {
		if sh := windowsShell(); sh != "" {
			return exec.CommandContext(ctx, sh, append([]string{filepath.ToSlash(path)}, args...)...) // #nosec G204 -- the handler path is validated by the caller.
		}
	}
	return exec.CommandContext(ctx, path, args...) // #nosec G204 -- the handler path is validated by the caller.
}

// ShellPath converts a filesystem path for use inside text that sh will parse.
// A POSIX shell treats backslashes as escapes, so on Windows C:\a\b becomes
// C:/a/b (which Windows accepts everywhere). Elsewhere it returns p unchanged.
func ShellPath(p string) string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(p)
	}
	return p
}

func nativeExecutable(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".bat", ".cmd", ".com":
		return true
	}
	return false
}

// windowsShell finds sh.exe: on PATH first, then in the default Git for Windows
// locations (git's own bin directory is often on PATH without usr\bin).
func windowsShell() string {
	if p, err := exec.LookPath("sh"); err == nil {
		return p
	}
	for _, env := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)"} {
		root := os.Getenv(env)
		if root == "" {
			continue
		}
		for _, rel := range []string{`Git\bin\sh.exe`, `Git\usr\bin\sh.exe`} {
			p := filepath.Join(root, rel)
			if st, err := os.Stat(p); err == nil && !st.IsDir() { // #nosec G703 -- probes fixed Git install locations under %ProgramFiles%; nothing is read.
				return p
			}
		}
	}
	return ""
}
