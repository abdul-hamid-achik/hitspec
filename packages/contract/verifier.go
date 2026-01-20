// Package contract provides contract testing support for hitspec.
package contract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/assertions"
	"github.com/abdul-hamid-achik/hitspec/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
)

// Verifier verifies contracts against a provider
type Verifier struct {
	providerURL  string
	stateHandler string // Path to state handler script
	verbose      bool
	client       *http.Client
	resolver     *env.Resolver
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
	file, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract file: %w", err)
	}

	return v.Verify(file)
}

// Verify verifies a parsed contract file
func (v *Verifier) Verify(file *parser.File) (*VerificationResult, error) {
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
		interactionResult := v.verifyInteraction(req, filepath.Dir(file.Path))
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

func (v *Verifier) verifyInteraction(req *parser.Request, baseDir string) InteractionResult {
	start := time.Now()
	result := InteractionResult{
		Name:        req.Name,
		Description: req.Description,
	}

	// Extract contract metadata from annotations
	if req.Metadata != nil {
		// Check for contract state annotation
		// The state would be stored in a custom annotation field
	}

	// Set up provider state if state handler is configured
	state := v.extractState(req)
	if state != "" {
		result.State = state
		if err := v.setupState(state); err != nil {
			result.Error = fmt.Errorf("failed to set up state %q: %w", state, err)
			result.Duration = time.Since(start)
			return result
		}
	}

	// Build and execute request
	httpReq := http.BuildRequestFromASTWithBaseDir(req, v.resolver.Resolve, baseDir)

	resp, err := v.client.Do(httpReq)
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
	// The parser would need to be extended to support custom annotations
	// For now, we'll look in the request description for state hints
	if req.Description != "" && strings.HasPrefix(req.Description, "state:") {
		return strings.TrimPrefix(req.Description, "state:")
	}
	return ""
}

// setupState sets up the provider state using the state handler script
func (v *Verifier) setupState(state string) error {
	if v.stateHandler == "" {
		if v.verbose {
			fmt.Printf("No state handler configured, skipping state setup for: %s\n", state)
		}
		return nil
	}

	if v.verbose {
		fmt.Printf("Setting up state: %s\n", state)
	}

	cmd := exec.Command(v.stateHandler, state)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VerifyDirectory verifies all contract files in a directory
func (v *Verifier) VerifyDirectory(dir string) ([]*VerificationResult, error) {
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

		result, err := v.VerifyFile(path)
		if err != nil {
			return err
		}

		results = append(results, result)
		return nil
	})

	return results, err
}
