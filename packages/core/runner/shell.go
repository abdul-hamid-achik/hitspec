package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// ShellResult represents the result of a shell command execution
type ShellResult struct {
	Command string
	Output  string
	Passed  bool
	Error   error
}

// executeShellCommands runs all shell commands for a request
func (r *Runner) executeShellCommands(commands []*parser.ShellCommand, baseDir string, resolver func(string) string) ([]*ShellResult, error) {
	if len(commands) == 0 {
		return nil, nil
	}

	var results []*ShellResult

	for _, cmd := range commands {
		result := r.executeShellCommand(cmd, baseDir, resolver)
		results = append(results, result)

		// If command failed and wasn't prefixed with "-", return error
		if !result.Passed && result.Error != nil {
			return results, result.Error
		}
	}

	return results, nil
}

// executeShellCommand executes a single shell command
func (r *Runner) executeShellCommand(cmd *parser.ShellCommand, baseDir string, resolver func(string) string) *ShellResult {
	result := &ShellResult{
		Command: cmd.Command,
		Passed:  true,
	}

	// Resolve variables in the command
	cmdStr := resolver(cmd.Command)
	cmdStr = strings.TrimSpace(cmdStr)

	if cmdStr == "" {
		return result
	}

	// Check if command should ignore errors (prefixed with "-")
	ignoreError := strings.HasPrefix(cmdStr, "-")
	if ignoreError {
		cmdStr = strings.TrimPrefix(cmdStr, "-")
		cmdStr = strings.TrimSpace(cmdStr)
	}

	// Execute via sh -c
	execCmd := exec.Command("sh", "-c", cmdStr)
	execCmd.Dir = baseDir
	execCmd.Env = os.Environ()

	output, err := execCmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Passed = ignoreError // Only pass if we're ignoring errors
		if !ignoreError {
			result.Error = fmt.Errorf("shell command failed: %s: %v\nOutput: %s", cmd.Command, err, output)
		}
	}

	if r.config.Verbose && len(output) > 0 {
		fmt.Printf("Shell output: %s\n", output)
	}

	return result
}
