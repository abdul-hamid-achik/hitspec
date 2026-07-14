package artifact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
)

const childOutputLimit = 1 << 20

const (
	maxReceiptIDBytes        = 128
	maxReceiptNameBytes      = 80
	maxReceiptStatusBytes    = 64
	maxReceiptTimestampBytes = 64
	maxReceiptHashBytes      = 128
	maxReceiptTags           = 32
	maxReceiptTagBytes       = 64
	maxReceiptFailures       = 16
	maxFailureStageBytes     = 64
	maxFailureErrorBytes     = 1024
)

// Fcheap invokes the file.cheap CLI through a fixed, operator-owned binary.
// Saves are serialized because file.cheap indexing has a single-writer lock.
type Fcheap struct {
	path     string
	stashDir string
	mu       sync.Mutex
	run      commandRunner
}

type commandRunner interface {
	Run(context.Context, string, []string, []string) ([]byte, []byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, path string, args, env []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = childOutputLimit, 16<<10
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if stdout.overflow || stderr.overflow {
		return stdout.Bytes(), stderr.Bytes(), errors.New("file.cheap output exceeded its bounded capture limit")
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.overflow = true
		return original, nil
	}
	if len(data) > remaining {
		b.overflow = true
		data = data[:remaining]
	}
	_, _ = b.Buffer.Write(data)
	return original, nil
}

// NewFcheap creates a sink. path must be a fixed executable chosen when the
// MCP server starts, never a model-supplied argument.
func NewFcheap(path, stashDir string) (*Fcheap, error) {
	path = strings.TrimSpace(path)
	stashDir = strings.TrimSpace(stashDir)
	if path == "" {
		return nil, errors.New("file.cheap executable path is required")
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("resolve file.cheap executable: %w", err)
	}
	if stashDir != "" {
		stashDir, err = filepath.Abs(stashDir)
		if err != nil {
			return nil, fmt.Errorf("resolve file.cheap stash directory: %w", err)
		}
	}
	return &Fcheap{path: resolved, stashDir: stashDir, run: execRunner{}}, nil
}

type fcheapFailure struct {
	ID    string `json:"id"`
	Stage string `json:"stage"`
	Error string `json:"error"`
}

type fcheapOutput struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	CreatedAt      string            `json:"created_at"`
	ContentHash    string            `json:"content_hash"`
	FileCount      int               `json:"file_count"`
	TotalSize      int64             `json:"total_size"`
	ExpiresAt      string            `json:"expires_at"`
	Tags           []string          `json:"tags"`
	Custom         map[string]string `json:"custom"`
	Status         string            `json:"status"`
	IndexRequested *bool             `json:"index_requested"`
	Indexed        bool              `json:"indexed"`
	Failed         []fcheapFailure   `json:"failed"`
}

// Save writes the content to a private temporary directory, hands only that
// fixed path to file.cheap, parses its durable receipt, then removes the handoff.
func (f *Fcheap) Save(ctx context.Context, input Input) (Receipt, error) {
	operationID := uuid.NewString()
	receipt := Receipt{Store: "fcheap", OperationID: operationID, Storage: StorageUnknown, Index: IndexSkipped}
	if ctx == nil {
		ctx = context.Background()
	}
	// ValidateTTL accepts surrounding whitespace for operator-facing config.
	// Normalize the by-value input before building argv so file.cheap receives
	// the same canonical value that Hitspec validated.
	input.TTL = strings.TrimSpace(input.TTL)
	if err := validateInput(input); err != nil {
		receipt.Storage = StorageFailed
		return receipt, err
	}

	dir, err := os.MkdirTemp("", "hitspec-artifact-*")
	if err != nil {
		receipt.Storage = StorageFailed
		return receipt, fmt.Errorf("create private artifact handoff: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup after the child exits
	if err := os.Chmod(dir, 0o700); err != nil {
		receipt.Storage = StorageFailed
		return receipt, fmt.Errorf("secure artifact handoff: %w", err)
	}
	artifactPath := filepath.Join(dir, input.Filename)
	if err := os.WriteFile(artifactPath, input.Content, 0o600); err != nil {
		receipt.Storage = StorageFailed
		return receipt, fmt.Errorf("write artifact handoff: %w", err)
	}

	args := []string{"--json", "--no-color"}
	if f.stashDir != "" {
		args = append(args, "--stash-dir", f.stashDir)
	}
	args = append(args, "save", dir, "--name", input.Name, "--tool", "hitspec", "--no-compress")
	for _, tag := range input.Tags {
		args = append(args, "--tag", tag)
	}
	if input.Source != "" {
		args = append(args, "--source", input.Source)
	}
	if input.TTL != "" {
		args = append(args, "--ttl", input.TTL)
	}
	if input.Index {
		args = append(args, "--index")
	}

	f.mu.Lock()
	stdout, stderr, runErr := f.run.Run(ctx, f.path, args, fcheapEnvironment())
	f.mu.Unlock()
	var output fcheapOutput
	decodeErr := json.Unmarshal(stdout, &output)
	stashID := cleanFieldLimit(output.ID, maxReceiptIDBytes)
	if decodeErr == nil && stashID != "" {
		receipt.Storage = StorageSucceeded
		receipt.StashID = stashID
		receipt.Name = cleanFieldLimit(output.Name, maxReceiptNameBytes)
		receipt.Status = cleanFieldLimit(output.Status, maxReceiptStatusBytes)
		receipt.CreatedAt = cleanFieldLimit(output.CreatedAt, maxReceiptTimestampBytes)
		receipt.ContentHash = cleanFieldLimit(output.ContentHash, maxReceiptHashBytes)
		receipt.ExpiresAt = cleanFieldLimit(output.ExpiresAt, maxReceiptTimestampBytes)
		receipt.Tags = cleanFields(output.Tags, maxReceiptTags, maxReceiptTagBytes)
		receipt.IndexRequested = copyBool(output.IndexRequested)
		if output.FileCount > 0 {
			receipt.FileCount = output.FileCount
		}
		if output.TotalSize > 0 {
			receipt.TotalSize = output.TotalSize
		}
		if secretsFound, parseErr := strconv.Atoi(output.Custom["secrets_found"]); parseErr == nil && secretsFound > 0 {
			receipt.SecretsFound = secretsFound
		}
		if input.Index {
			if output.Indexed {
				receipt.Index = IndexSucceeded
			} else {
				receipt.Index = IndexFailed
			}
		}
		for _, failure := range output.Failed[:min(len(output.Failed), maxReceiptFailures)] {
			cleaned := Failure{
				ID:    cleanFieldLimit(failure.ID, maxReceiptIDBytes),
				Stage: cleanFieldLimit(failure.Stage, maxFailureStageBytes),
				Error: cleanFieldLimit(failure.Error, maxFailureErrorBytes),
			}
			if cleaned.ID != "" || cleaned.Stage != "" || cleaned.Error != "" {
				receipt.Failures = append(receipt.Failures, cleaned)
			}
		}
		// A non-zero exit after a parseable receipt is a known partial outcome.
		// Returning the receipt avoids a duplicate save; failed indexing remains explicit.
		return receipt, nil
	}
	if runErr == nil {
		runErr = decodeErr
	}
	if runErr == nil {
		runErr = errors.New("file.cheap receipt omitted a stash id")
	}
	if safe := cleanField(string(stderr)); safe != "" {
		runErr = fmt.Errorf("%w: %s", runErr, safe)
	}
	return receipt, &OutcomeUnknownError{OperationID: operationID, Err: runErr}
}

func validateInput(input Input) error {
	if input.Filename == "" || filepath.Base(input.Filename) != input.Filename || input.Filename == "." || input.Filename == ".." {
		return errors.New("artifact filename must be a safe basename")
	}
	if len(input.Content) == 0 {
		return errors.New("artifact content is empty")
	}
	if len(input.Name) == 0 || len(input.Name) > maxReceiptNameBytes || strings.ContainsAny(input.Name, "\r\n") {
		return errors.New("artifact name must be one line and at most 80 bytes")
	}
	if err := ValidateTTL(input.TTL); err != nil {
		return err
	}
	if len(input.Tags) > maxReceiptTags {
		return fmt.Errorf("artifact tags must contain at most %d values", maxReceiptTags)
	}
	for _, tag := range input.Tags {
		if tag == "" || len(tag) > maxReceiptTagBytes || strings.ContainsAny(tag, "\r\n") || strings.HasPrefix(tag, "-") {
			return fmt.Errorf("invalid artifact tag %q", tag)
		}
	}
	return nil
}

// fcheapEnvironment deliberately excludes provider credentials inherited by
// Hitspec. file.cheap receives only its own configuration and basic process env.
func fcheapEnvironment() []string {
	allowed := map[string]bool{
		"HOME": true, "PATH": true, "TMPDIR": true, "XDG_CONFIG_HOME": true,
		"XDG_DATA_HOME": true, "NO_COLOR": true, "LANG": true,
	}
	var env []string
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && (allowed[key] || strings.HasPrefix(key, "LC_") || strings.HasPrefix(key, "FCHEAP_")) {
			env = append(env, item)
		}
	}
	return env
}

func cleanField(value string) string {
	return cleanFieldLimit(value, maxFailureErrorBytes)
}

func cleanFieldLimit(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > limit {
		value = value[:limit]
		// Avoid returning malformed UTF-8 when the byte limit lands within a
		// multibyte rune. encoding/json repairs invalid input before this point.
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func cleanFields(values []string, maxValues, maxBytes int) []string {
	if len(values) > maxValues {
		values = values[:maxValues]
	}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = cleanFieldLimit(value, maxBytes); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func copyBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
