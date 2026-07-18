package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/mcpclient"
	"github.com/spf13/cobra"
)

const maxMCPArgumentsBytes = 1 << 20

type mcpClientFlags struct {
	url     string
	headers []string
	env     []string
	cwd     string
	timeout time.Duration
	json    bool
}

func newMCPProbeCommand() *cobra.Command {
	var flags mcpClientFlags
	var requiredTools []string
	command := &cobra.Command{
		Use:          "probe [-- <server-command> [args...]]",
		Short:        "Test an MCP handshake and discover all tools",
		SilenceUsage: true,
		Long: `Connect to an MCP server, negotiate the protocol, traverse every tools/list
page, and optionally require named tools. Select exactly one target: pass a
Streamable HTTP endpoint with --url, or put a stdio server argv after --.

Examples:
  hitspec mcp probe --require-tool echo -- ./my-mcp-server
  hitspec mcp probe --url http://127.0.0.1:3000/mcp --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if flags.timeout <= 0 {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", "--timeout must be greater than zero")
			}
			target, err := flags.target(args, command.ErrOrStderr())
			if err != nil {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", err.Error())
			}
			ctx, cancel := context.WithTimeout(command.Context(), flags.timeout)
			defer cancel()
			client, err := mcpclient.Connect(ctx, target, version)
			if err != nil {
				return writeMCPConnectionError(command, flags.json, err)
			}
			report, operationErr := client.Probe(ctx, requiredTools)
			closeErr := client.Close()
			if operationErr != nil {
				return writeMCPConnectionError(command, flags.json, operationErr)
			}
			if closeErr != nil {
				return writeMCPConnectionError(command, flags.json, fmt.Errorf("close MCP session: %w", closeErr))
			}
			if flags.json {
				if err := mcpclient.WriteJSON(command.OutOrStdout(), report); err != nil {
					return err
				}
			} else if err := mcpclient.WriteProbeHuman(command.OutOrStdout(), report); err != nil {
				return err
			}
			if !report.OK {
				return newExitError(
					ExitTestFailure,
					"MCP probe failed: missing required tools: "+strings.Join(report.MissingTools, ", "),
				)
			}
			return nil
		},
	}
	flags.bind(command)
	command.Flags().StringSliceVar(&requiredTools, "require-tool", nil, "Require a tool name (repeatable or comma-separated)")
	return command
}

func newMCPCallCommand() *cobra.Command {
	var flags mcpClientFlags
	var argumentsSource string
	command := &cobra.Command{
		Use:          "call <tool> [-- <server-command> [args...]]",
		Short:        "Call one tool on an MCP server",
		SilenceUsage: true,
		Long: `Connect to an MCP server and invoke one advertised tool. Arguments must be a
JSON object, supplied inline or from a file with @path. A result with isError
set is printed and exits as a failed test.

Examples:
  hitspec mcp call echo --args '{"message":"hello"}' -- ./my-mcp-server
  hitspec mcp call weather --args @weather.json --url https://example.com/mcp --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if flags.timeout <= 0 {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", "--timeout must be greater than zero")
			}
			if len(args) == 0 {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", "tool name is required")
			}
			toolName := strings.TrimSpace(args[0])
			if toolName == "" {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", "tool name must not be empty")
			}
			target, err := flags.target(args[1:], command.ErrOrStderr())
			if err != nil {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", err.Error())
			}
			arguments, err := readMCPArguments(argumentsSource)
			if err != nil {
				return writeMCPError(command, flags.json, ExitUsageError, "usage", err.Error())
			}

			ctx, cancel := context.WithTimeout(command.Context(), flags.timeout)
			defer cancel()
			client, err := mcpclient.Connect(ctx, target, version)
			if err != nil {
				return writeMCPConnectionError(command, flags.json, err)
			}
			report, operationErr := client.Call(ctx, toolName, arguments)
			closeErr := client.Close()
			if operationErr != nil {
				if errors.Is(operationErr, mcpclient.ErrToolNotFound) ||
					errors.Is(operationErr, mcpclient.ErrContractViolation) {
					return writeMCPError(command, flags.json, ExitTestFailure, "test", operationErr.Error())
				}
				return writeMCPConnectionError(command, flags.json, operationErr)
			}
			if closeErr != nil {
				return writeMCPConnectionError(command, flags.json, fmt.Errorf("close MCP session: %w", closeErr))
			}
			if flags.json {
				if err := mcpclient.WriteJSON(command.OutOrStdout(), report); err != nil {
					return err
				}
			} else if err := mcpclient.WriteCallHuman(command.OutOrStdout(), report); err != nil {
				return err
			}
			if report.IsError {
				return newExitError(ExitTestFailure, fmt.Sprintf("MCP tool %q reported an execution error", toolName))
			}
			return nil
		},
	}
	flags.bind(command)
	command.Flags().StringVar(&argumentsSource, "args", "{}", "Tool arguments as a JSON object or @path")
	return command
}

func (f *mcpClientFlags) bind(command *cobra.Command) {
	command.Flags().StringVar(&f.url, "url", "", "Streamable HTTP MCP endpoint")
	command.Flags().StringArrayVar(&f.headers, "header", nil, "HTTP header as 'Name: Value' (repeatable)")
	command.Flags().StringArrayVar(&f.env, "env", nil, "Stdio environment override as KEY=VALUE (repeatable)")
	command.Flags().StringVar(&f.cwd, "cwd", "", "Working directory for a stdio server")
	command.Flags().DurationVar(&f.timeout, "timeout", 30*time.Second, "Maximum duration of the MCP operation")
	command.Flags().BoolVar(&f.json, "json", false, "Emit a stable JSON report")
}

func (f *mcpClientFlags) target(commandArgs []string, stderr io.Writer) (mcpclient.Target, error) {
	target := mcpclient.Target{
		URL:     strings.TrimSpace(f.url),
		Command: commandArgs,
		Dir:     f.cwd,
		Env:     f.env,
		Headers: f.headers,
		Stderr:  stderr,
	}
	if err := target.Validate(); err != nil {
		return mcpclient.Target{}, err
	}
	return target, nil
}

func readMCPArguments(source string) (map[string]any, error) {
	data := []byte(source)
	if strings.HasPrefix(source, "@") {
		path := strings.TrimPrefix(source, "@")
		if path == "" {
			return nil, errors.New("--args @path requires a file path")
		}
		file, err := os.Open(path) // #nosec G304 -- @path is an explicit user-selected arguments file, not a workspace-relative input.
		if err != nil {
			return nil, fmt.Errorf("read MCP arguments file: %w", err)
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, maxMCPArgumentsBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read MCP arguments file: %w", err)
		}
		if len(data) > maxMCPArgumentsBytes {
			return nil, fmt.Errorf("MCP arguments exceed the %d-byte limit", maxMCPArgumentsBytes)
		}
	}
	var arguments map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&arguments); err != nil {
		return nil, fmt.Errorf("MCP arguments must be a valid JSON object: %w", err)
	}
	if arguments == nil {
		return nil, errors.New("MCP arguments must be a JSON object, not null")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("MCP arguments must contain exactly one JSON object")
		}
		return nil, fmt.Errorf("MCP arguments must contain exactly one JSON object: %w", err)
	}
	return arguments, nil
}

type mcpErrorReport struct {
	OK    bool           `json:"ok"`
	Error mcpErrorDetail `json:"error"`
}

type mcpErrorDetail struct {
	Code    int    `json:"code"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func writeMCPConnectionError(command *cobra.Command, jsonOutput bool, err error) error {
	message := err.Error()
	if errors.Is(err, context.DeadlineExceeded) {
		message = "MCP operation timed out"
	}
	return writeMCPError(command, jsonOutput, ExitNetworkError, "connection", message)
}

func writeMCPError(command *cobra.Command, jsonOutput bool, code int, kind, message string) error {
	message = safeMCPDiagnostic(message)
	if jsonOutput {
		report := mcpErrorReport{
			OK: false,
			Error: mcpErrorDetail{
				Code:    code,
				Kind:    kind,
				Message: message,
			},
		}
		if err := mcpclient.WriteJSON(command.OutOrStdout(), report); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(command.ErrOrStderr(), "Error: %s\n", message); err != nil {
		return err
	}
	return newExitError(code, message)
}

func safeMCPDiagnostic(message string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < ' ' || r == '\x7f' {
			return -1
		}
		return r
	}, message))
}
