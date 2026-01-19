package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// executePreHooks runs all pre-hooks for a request
func (r *Runner) executePreHooks(hooks []*parser.Hook, baseDir string, resolver func(string) string) error {
	for _, hook := range hooks {
		if hook.Type != parser.HookExec {
			continue
		}
		if err := r.executeHook(hook, baseDir, resolver); err != nil {
			return fmt.Errorf("pre-hook failed: %w", err)
		}
	}
	return nil
}

// executePostHooks runs all post-hooks for a request
func (r *Runner) executePostHooks(hooks []*parser.Hook, baseDir string, resolver func(string) string) error {
	var firstErr error
	for _, hook := range hooks {
		if hook.Type != parser.HookExec {
			continue
		}
		if err := r.executeHook(hook, baseDir, resolver); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("post-hook failed: %w", err)
			}
			// Continue executing other post-hooks even if one fails
			// since post-hooks are typically cleanup tasks
		}
	}
	return firstErr
}

// executeHook executes a single hook command
func (r *Runner) executeHook(hook *parser.Hook, baseDir string, resolver func(string) string) error {
	// Resolve variables in the command
	cmdStr := resolver(hook.Command)
	cmdStr = strings.TrimSpace(cmdStr)

	if cmdStr == "" {
		return nil
	}

	// Handle relative paths - if the command starts with ./ or ../ or is just a filename
	// that exists in baseDir, make it relative to baseDir
	parts := strings.Fields(cmdStr)
	if len(parts) > 0 {
		executable := parts[0]
		if strings.HasPrefix(executable, "./") || strings.HasPrefix(executable, "../") {
			// Relative path - join with baseDir
			parts[0] = filepath.Join(baseDir, executable)
			cmdStr = strings.Join(parts, " ")
		} else if !filepath.IsAbs(executable) && !isInPath(executable) {
			// Check if it's a script in the base directory
			potentialPath := filepath.Join(baseDir, executable)
			if _, err := os.Stat(potentialPath); err == nil {
				parts[0] = potentialPath
				cmdStr = strings.Join(parts, " ")
			}
		}
	}

	// Execute the command using sh -c for proper shell handling
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = baseDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %q failed: %v\nOutput: %s", hook.Command, err, string(output))
	}

	if r.config.Verbose && len(output) > 0 {
		fmt.Fprintf(os.Stderr, "Hook output: %s\n", string(output))
	}

	return nil
}

// isInPath checks if a command is available in the system PATH
func isInPath(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
