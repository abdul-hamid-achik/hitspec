package cmd

import (
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/fetch"
	"github.com/spf13/cobra"
)

type fetchCLIOptions struct {
	method       string
	headers      []string
	data         string
	format       string
	outputFile   string
	force        bool
	fail         bool
	timeout      string
	maxBytes     int64
	maxRedirects int
	noFollow     bool
	insecure     bool
	proxy        string
	environment  string
	envFile      string
	config       string
	name         string
	index        int
}

var fetchCmd = newFetchCommand()

func newFetchCommand() *cobra.Command {
	options := &fetchCLIOptions{}
	command := &cobra.Command{
		Use:   "fetch <url|file>",
		Short: "Fetch one response body",
		Long: `Execute one ad-hoc URL or one request from a .http/.hitspec file.

Raw output is byte-exact. Text extracts readable content, Markdown creates a
self-contained response document, and JSON returns a machine-safe envelope.
Payload is written only to stdout or --output-file; diagnostics use stderr.

Examples:
  hitspec fetch https://example.com/data
  hitspec fetch https://example.com/docs --format markdown
  hitspec fetch api.http --name getUsers --env staging --format text
  hitspec fetch https://example.com/archive --output-file archive.bin`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetchCommand(cmd, args[0], options)
		},
	}
	flags := command.Flags()
	flags.StringVarP(&options.method, "request", "X", "", "HTTP method for an ad-hoc URL")
	flags.StringArrayVarP(&options.headers, "header", "H", nil, "Request header (repeatable, Name: value)")
	flags.StringVarP(&options.data, "data", "d", "", "Request body for an ad-hoc URL")
	flags.StringVar(&options.format, "format", string(fetch.FormatRaw), "Response format: raw, text, markdown, json")
	flags.StringVarP(&options.outputFile, "output-file", "o", "", "Write payload to a file instead of stdout")
	flags.BoolVar(&options.force, "force", false, "Replace an existing regular output file")
	flags.BoolVarP(&options.fail, "fail", "f", false, "Return an error for non-2xx HTTP status")
	flags.StringVar(&options.timeout, "timeout", "", "Request timeout (for example 30s or 2m)")
	flags.Int64Var(&options.maxBytes, "max-bytes", fetch.DefaultMaxBodyBytes, "Maximum response body size in bytes")
	flags.IntVar(&options.maxRedirects, "max-redirects", fetch.DefaultMaxRedirects, "Maximum redirects to follow")
	flags.BoolVar(&options.noFollow, "no-follow", false, "Do not follow redirects")
	flags.BoolVarP(&options.insecure, "insecure", "k", false, "Disable TLS certificate validation")
	flags.StringVar(&options.proxy, "proxy", "", "HTTP proxy URL")
	flags.StringVarP(&options.environment, "env", "e", "", "Environment for a saved request (default: config, then dev)")
	flags.StringVar(&options.envFile, "env-file", "", "Dotenv file for a saved request")
	flags.StringVar(&options.config, "config", "", "Hitspec config file for a saved request")
	flags.StringVarP(&options.name, "name", "n", "", "Saved request name")
	flags.IntVar(&options.index, "index", 0, "Saved request number (1-based)")
	return command
}

func runFetchCommand(command *cobra.Command, source string, options *fetchCLIOptions) error {
	format, err := fetch.ParseFormat(options.format)
	if err != nil {
		return err
	}
	if options.maxBytes <= 0 {
		return errors.New("--max-bytes must be greater than zero")
	}
	if options.maxRedirects <= 0 {
		return errors.New("--max-redirects must be greater than zero")
	}
	timeout, err := parseFetchTimeout(options.timeout)
	if err != nil {
		return err
	}

	var result *fetch.Result
	if isHTTPURL(source) {
		result, err = fetchDirectURL(command.Context(), source, options, timeout)
	} else {
		if command.Flags().Changed("request") || command.Flags().Changed("header") || command.Flags().Changed("data") {
			return errors.New("--request, --header, and --data apply only when the source is a URL")
		}
		result, err = fetchSavedRequest(command.Context(), source, options, timeout)
	}
	if err != nil {
		return err
	}
	payload, err := fetch.Render(command.Context(), result, format)
	if err != nil {
		return err
	}
	if err := writeFetchPayload(command, options.outputFile, payload, options.force); err != nil {
		return err
	}
	if options.fail && !result.Success() {
		return fmt.Errorf("HTTP request returned %s", result.Status)
	}
	return nil
}

func fetchDirectURL(ctx context.Context, source string, options *fetchCLIOptions, timeout time.Duration) (*fetch.Result, error) {
	headers, err := parseFetchHeaders(options.headers)
	if err != nil {
		return nil, err
	}
	method := options.method
	if method == "" && options.data != "" {
		method = stdhttp.MethodPost
	}
	return fetch.NewService().Fetch(ctx, fetch.Request{
		Method: method, URL: source, Headers: headers, Body: []byte(options.data), Timeout: timeout,
		FollowRedirects: !options.noFollow, MaxRedirects: options.maxRedirects,
		Insecure: options.insecure, Proxy: options.proxy, MaxBodyBytes: options.maxBytes,
		UserAgent: "hitspec/" + version, NetworkPolicy: fetch.NetworkAny,
	})
}

func fetchSavedRequest(ctx context.Context, path string, options *fetchCLIOptions, timeoutOverride time.Duration) (*fetch.Result, error) {
	return fetch.FetchSaved(ctx, fetch.SavedRequest{
		Path: path, Name: options.name, Index: options.index,
		Environment: options.environment, EnvFile: options.envFile, ConfigFile: options.config,
		Timeout: timeoutOverride, MaxBodyBytes: options.maxBytes, MaxRedirects: options.maxRedirects, NoFollow: options.noFollow,
		Insecure: options.insecure, Proxy: options.proxy, DefaultUserAgent: "hitspec/" + version,
	})
}

func parseFetchHeaders(values []string) (stdhttp.Header, error) {
	headers := make(stdhttp.Header)
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" || strings.ContainsAny(name, " \t\r\n") {
			return nil, fmt.Errorf("invalid header %q (expected Name: value)", raw)
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("invalid header %q (newlines are not allowed)", name)
		}
		headers.Add(name, strings.TrimSpace(value))
	}
	return headers, nil
}

func parseFetchTimeout(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --timeout %q (expected a positive duration such as 30s)", value)
	}
	return duration, nil
}

func isHTTPURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func writeFetchPayload(command *cobra.Command, path string, payload []byte, force bool) error {
	if path == "" {
		_, err := command.OutOrStdout().Write(payload)
		return err
	}
	return atomicWriteFetchFile(path, payload, force)
}

func atomicWriteFetchFile(path string, payload []byte, force bool) error {
	directory := filepath.Dir(path)
	info, statErr := os.Lstat(path)
	if statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to replace symlink %s", path)
		}
		if !force {
			return fmt.Errorf("output file %s already exists (use --force to replace it)", path)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect output file: %w", statErr)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary output file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure temporary output file: %w", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		temporary.Close()
		return fmt.Errorf("write output file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync output file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close output file: %w", err)
	}
	if force {
		if err := os.Rename(temporaryPath, path); err != nil {
			return fmt.Errorf("replace output file: %w", err)
		}
		return nil
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	return nil
}
