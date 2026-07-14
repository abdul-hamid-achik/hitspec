package artifact

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// StorageState records whether durable storage is known to have happened.
type StorageState string

const (
	StorageSucceeded StorageState = "succeeded"
	StorageFailed    StorageState = "failed"
	StorageUnknown   StorageState = "unknown"
)

// IndexState records the independent file.cheap indexing outcome.
type IndexState string

const (
	IndexSucceeded IndexState = "succeeded"
	IndexSkipped   IndexState = "skipped"
	IndexFailed    IndexState = "failed"
)

// Failure is one post-save operation that did not complete.
type Failure struct {
	ID    string `json:"id,omitempty"`
	Stage string `json:"stage"`
	Error string `json:"error"`
}

// Receipt is the bounded, provider-neutral result of an artifact save.
type Receipt struct {
	Store          string       `json:"store"`
	OperationID    string       `json:"operation_id"`
	Storage        StorageState `json:"storage"`
	Index          IndexState   `json:"index"`
	StashID        string       `json:"stash_id,omitempty"`
	Name           string       `json:"name,omitempty"`
	Status         string       `json:"status,omitempty"`
	CreatedAt      string       `json:"created_at,omitempty"`
	ContentHash    string       `json:"content_hash,omitempty"`
	FileCount      int          `json:"file_count,omitempty"`
	TotalSize      int64        `json:"total_size,omitempty"`
	ExpiresAt      string       `json:"expires_at,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
	IndexRequested *bool        `json:"index_requested,omitempty"`
	SecretsFound   int          `json:"secrets_found,omitempty"`
	Failures       []Failure    `json:"failures,omitempty"`
}

// Input is one already-rendered artifact. Filename is a fixed basename, never
// a caller-selected destination path.
type Input struct {
	Filename string
	Content  []byte
	Name     string
	Source   string
	Tags     []string
	TTL      string
	Index    bool
}

// Sink stores one rendered artifact and returns an independently meaningful
// storage/index receipt.
type Sink interface {
	Save(context.Context, Input) (Receipt, error)
}

// OutcomeUnknownError means the child process may have persisted the artifact
// but did not produce a receipt. Callers must inspect before retrying.
type OutcomeUnknownError struct {
	OperationID string
	Err         error
}

func (e *OutcomeUnknownError) Error() string {
	return fmt.Sprintf("artifact outcome unknown for operation %s: %v", e.OperationID, e.Err)
}

func (e *OutcomeUnknownError) Unwrap() error { return e.Err }

// IsOutcomeUnknown reports whether a save needs explicit reconciliation.
func IsOutcomeUnknown(err error) bool {
	var target *OutcomeUnknownError
	return errors.As(err, &target)
}

var ttlPattern = regexp.MustCompile(`^(?:[1-9][0-9]{0,5}[hdwmy]|[0-9]{4}-[0-9]{2}-[0-9]{2})$`)

// ValidateTTL accepts the bounded duration/date forms supported by file.cheap.
func ValidateTTL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 16 || !ttlPattern.MatchString(value) {
		return errors.New("artifact ttl must be a positive h/d/w/m/y duration or YYYY-MM-DD")
	}
	if strings.Contains(value, "-") {
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			return errors.New("artifact ttl must be a valid calendar date in YYYY-MM-DD form")
		}
	}
	return nil
}
