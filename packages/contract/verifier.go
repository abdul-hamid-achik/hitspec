// Package contract provides contract testing support for hitspec.
package contract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// Verifier verifies contracts against a provider
type Verifier struct {
	providerURL  string
	stateHandler string // Path to state handler script
	verbose      bool
	client       *http.Client
	resolver     *env.Resolver
	fileRoot     string // when set, `< file` bodies must live under it
}

// Option is a functional option for Verifier
type Option func(*Verifier)

// WithProviderURL sets the provider URL
func WithProviderURL(url string) Option {
	return func(v *Verifier) {
		v.providerURL = url
	}
}

// WithStateHandler sets the state handler script path
func WithStateHandler(path string) Option {
	return func(v *Verifier) {
		v.stateHandler = path
	}
}

// WithVerbose enables verbose output
func WithVerbose(verbose bool) Option {
	return func(v *Verifier) {
		v.verbose = verbose
	}
}

// WithHTTPClient sets the client used to call the provider. By default the
// verifier uses a client with the http package defaults; pass one built from
// the run configuration (runconfig.Build) to honor hitspec.yaml.
func WithHTTPClient(client *http.Client) Option {
	return func(v *Verifier) {
		if client != nil {
			v.client = client
		}
	}
}

// WithFileRoot confines `< file` bodies and multipart files of the contract's
// requests to root (the workspace of serve/studio); a request that points
// outside fails its interaction without being sent. Empty disables the check,
// which is the CLI's behavior.
func WithFileRoot(root string) Option {
	return func(v *Verifier) {
		v.fileRoot = root
	}
}

// NewVerifier creates a new contract verifier
func NewVerifier(opts ...Option) *Verifier {
	v := &Verifier{
		client:   http.NewClient(),
		resolver: env.NewResolver(),
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// VerificationResult represents the result of verifying a contract
type VerificationResult struct {
	ContractFile string
	Passed       int
	Failed       int
	Skipped      int
	Results      []InteractionResult
	Duration     time.Duration
}

// InteractionResult represents the result of verifying a single interaction
type InteractionResult struct {
	Name        string
	Description string
	Provider    string
	State       string
	Passed      bool
	Error       error
	Assertions  []*assertions.Result
	Duration    time.Duration
}

// VerifyFile verifies a contract file against the provider
func (v *Verifier) VerifyFile(path string) (*VerificationResult, error) {
	return v.VerifyFileContext(context.Background(), path)
}

// VerifyFileContext is VerifyFile bound to ctx: cancellation aborts the
// in-flight provider call and stops before the next interaction.
func (v *Verifier) VerifyFileContext(ctx context.Context, path string) (*VerificationResult, error) {
	file, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract file: %w", err)
	}

	return v.VerifyContext(ctx, file)
}

// Verify verifies a parsed contract file
func (v *Verifier) Verify(file *parser.File) (*VerificationResult, error) {
	return v.VerifyContext(context.Background(), file)
}

// VerifyContext is Verify bound to ctx. When ctx ends, the interactions that
// did not run are left out of the result and the context error is returned
// alongside the partial result.
func (v *Verifier) VerifyContext(ctx context.Context, file *parser.File) (*VerificationResult, error) {
	start := time.Now()

	result := &VerificationResult{
		ContractFile: file.Path,
		Results:      make([]InteractionResult, 0),
	}

	// Set up variables
	for _, variable := range file.Variables {
		v.resolver.SetVariable(variable.Name, variable.Value)
	}

	// Override baseUrl with provider URL
	if v.providerURL != "" {
		v.resolver.SetVariable("baseUrl", v.providerURL)
	}

	// Process each request as a contract interaction
	for _, req := range file.Requests {
		if err := ctx.Err(); err != nil {
			result.Duration = time.Since(start)
			return result, err
		}
		interactionResult := v.verifyInteraction(ctx, req, filepath.Dir(file.Path))
		result.Results = append(result.Results, interactionResult)

		if interactionResult.Passed {
			result.Passed++
		} else if interactionResult.Error != nil && strings.Contains(interactionResult.Error.Error(), "skipped") {
			result.Skipped++
		} else {
			result.Failed++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

func (v *Verifier) verifyInteraction(ctx context.Context, req *parser.Request, baseDir string) InteractionResult {
	start := time.Now()
	result := InteractionResult{
		Name:        req.Name,
		Description: req.Description,
	}

	// Extract contract metadata from custom annotations
	if req.Metadata != nil && req.Metadata.Custom != nil {
		if provider, ok := req.Metadata.Custom["contract.provider"]; ok {
			result.Provider = provider
		}
	}

	// Set up provider state if state handler is configured
	state := v.extractState(req)
	if state != "" {
		result.State = state
		if err := v.setupStateContext(ctx, state); err != nil {
			result.Error = fmt.Errorf("failed to set up state %q: %w", state, err)
			result.Duration = time.Since(start)
			return result
		}
	}

	if v.fileRoot != "" {
		if err := http.CheckFileRefs(req, v.resolver.Resolve, baseDir, v.fileRoot); err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result
		}
	}

	// Build and execute request
	httpReq := http.BuildRequestFromASTWithBaseDir(req, v.resolver.Resolve, baseDir)

	resp, err := v.client.DoWithContext(ctx, httpReq)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Evaluate assertions
	if len(req.Assertions) > 0 {
		result.Assertions = assertions.EvaluateAllWithBaseDir(resp, req.Assertions, baseDir)
		result.Passed = true
		for _, a := range result.Assertions {
			if !a.Passed {
				result.Passed = false
				break
			}
		}
	} else {
		// Default: check for success status
		result.Passed = resp.IsSuccess()
	}

	result.Duration = time.Since(start)
	return result
}

// extractState extracts the contract state from request metadata
// Looking for # @contract.state "state description" annotation
func (v *Verifier) extractState(req *parser.Request) string {
	// First, check custom annotations for contract.state
	if req.Metadata != nil && req.Metadata.Custom != nil {
		if state, ok := req.Metadata.Custom["contract.state"]; ok {
			return state
		}
	}
	// Fallback: look in the request description for state hints
	if req.Description != "" && strings.HasPrefix(req.Description, "state:") {
		return strings.TrimPrefix(req.Description, "state:")
	}
	return ""
}

// setupState sets up the provider state using the state handler script
func (v *Verifier) setupState(state string) error {
	return v.setupStateContext(context.Background(), state)
}

// setupStateContext is setupState bound to ctx so a cancelled run also stops
// the state handler.
func (v *Verifier) setupStateContext(ctx context.Context, state string) error {
	if v.stateHandler == "" {
		if v.verbose {
			fmt.Printf("No state handler configured, skipping state setup for: %s\n", state)
		}
		return nil
	}

	if v.verbose {
		fmt.Printf("Setting up state: %s\n", state)
	}

	cmd := exec.CommandContext(ctx, v.stateHandler, state)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VerifyDirectory verifies all contract files in a directory
func (v *Verifier) VerifyDirectory(dir string) ([]*VerificationResult, error) {
	return v.VerifyDirectoryContext(context.Background(), dir)
}

// VerifyDirectoryContext is VerifyDirectory bound to ctx.
func (v *Verifier) VerifyDirectoryContext(ctx context.Context, dir string) ([]*VerificationResult, error) {
	var results []*VerificationResult

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".http" && ext != ".hitspec" {
			return nil
		}

		result, err := v.VerifyFileContext(ctx, path)
		if err != nil {
			return err
		}

		results = append(results, result)
		return nil
	})

	return results, err
}
