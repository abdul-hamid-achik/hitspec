// Package snapshot provides snapshot testing functionality for hitspec.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	// SnapshotDir is the directory name for storing snapshots
	SnapshotDir = "__snapshots__"
	// SnapshotExt is the file extension for snapshot files
	SnapshotExt = ".snap.json"
)

// Manager handles snapshot storage and comparison.
type Manager struct {
	baseDir       string
	updateMode    bool
	snapshotsRead map[string]map[string]any // file -> {name -> value}
}

// NewManager creates a new snapshot manager.
func NewManager(baseDir string, updateMode bool) *Manager {
	return &Manager{
		baseDir:       baseDir,
		updateMode:    updateMode,
		snapshotsRead: make(map[string]map[string]any),
	}
}

// SnapshotResult represents the result of a snapshot comparison.
type SnapshotResult struct {
	Passed     bool
	Message    string
	Expected   any
	Actual     any
	IsNew      bool
	WasUpdated bool
}

// Compare compares an actual value against a stored snapshot.
// If updateMode is true and there's a mismatch, the snapshot is updated.
// The name parameter is optional; if empty, a hash of the value is used.
func (m *Manager) Compare(testFile string, requestName string, snapshotName string, actual any) *SnapshotResult {
	result := &SnapshotResult{
		Actual: actual,
	}

	// Determine snapshot file path
	snapshotFile := m.getSnapshotFilePath(testFile)

	// Generate snapshot key
	key := m.generateKey(requestName, snapshotName, actual)

	// Load existing snapshots for this file
	snapshots, err := m.loadSnapshots(snapshotFile)
	if err != nil && !os.IsNotExist(err) {
		result.Passed = false
		result.Message = fmt.Sprintf("failed to load snapshots: %v", err)
		return result
	}

	// Check if snapshot exists
	expected, exists := snapshots[key]
	if !exists {
		if m.updateMode {
			// Create new snapshot
			snapshots[key] = actual
			if err := m.saveSnapshots(snapshotFile, snapshots); err != nil {
				result.Passed = false
				result.Message = fmt.Sprintf("failed to save snapshot: %v", err)
				return result
			}
			result.Passed = true
			result.IsNew = true
			result.Expected = actual
			result.Message = "new snapshot created"
			return result
		}

		// Snapshot doesn't exist and not in update mode
		result.Passed = false
		result.Message = "snapshot does not exist (run with --update-snapshots to create)"
		return result
	}

	result.Expected = expected

	// Compare values
	if m.deepEqual(expected, actual) {
		result.Passed = true
		return result
	}

	// Mismatch
	if m.updateMode {
		// Update snapshot
		snapshots[key] = actual
		if err := m.saveSnapshots(snapshotFile, snapshots); err != nil {
			result.Passed = false
			result.Message = fmt.Sprintf("failed to update snapshot: %v", err)
			return result
		}
		result.Passed = true
		result.WasUpdated = true
		result.Message = "snapshot updated"
		return result
	}

	result.Passed = false
	result.Message = "snapshot mismatch"
	return result
}

// getSnapshotFilePath returns the path to the snapshot file for a test file.
func (m *Manager) getSnapshotFilePath(testFile string) string {
	dir := filepath.Dir(testFile)
	base := filepath.Base(testFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	return filepath.Join(dir, SnapshotDir, name+SnapshotExt)
}

// generateKey generates a unique key for a snapshot.
func (m *Manager) generateKey(requestName, snapshotName string, value any) string {
	if snapshotName != "" {
		return fmt.Sprintf("%s::%s", requestName, snapshotName)
	}
	if requestName != "" {
		return requestName
	}
	// If no name, use hash of the request context
	hash := sha256.Sum256([]byte(fmt.Sprintf("%v", value)))
	return "anon_" + hex.EncodeToString(hash[:8])
}

// loadSnapshots loads snapshots from a file.
func (m *Manager) loadSnapshots(path string) (map[string]any, error) {
	// Check cache first
	if cached, ok := m.snapshotsRead[path]; ok {
		return cached, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}

	var snapshots map[string]any
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, err
	}

	m.snapshotsRead[path] = snapshots
	return snapshots, nil
}

// saveSnapshots saves snapshots to a file.
func (m *Manager) saveSnapshots(path string, snapshots map[string]any) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return err
	}

	// Update cache
	m.snapshotsRead[path] = snapshots

	return os.WriteFile(path, data, 0644)
}

// deepEqual compares two values for deep equality.
func (m *Manager) deepEqual(a, b any) bool {
	// Handle JSON number comparisons
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)

	var aVal, bVal any
	if err := json.Unmarshal(aJSON, &aVal); err == nil {
		a = aVal
	}
	if err := json.Unmarshal(bJSON, &bVal); err == nil {
		b = bVal
	}

	return reflect.DeepEqual(a, b)
}

// Snapshot is a convenience type for JSON-serializable snapshot data.
type Snapshot struct {
	Version  int            `json:"version"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Data     map[string]any `json:"data"`
}

// Global snapshot manager (initialized per test run)
var globalManager *Manager

// SetGlobalManager sets the global snapshot manager.
func SetGlobalManager(m *Manager) {
	globalManager = m
}

// GetGlobalManager returns the global snapshot manager.
func GetGlobalManager() *Manager {
	return globalManager
}

// CompareSnapshot is a convenience function for comparing snapshots using the global manager.
func CompareSnapshot(testFile, requestName, snapshotName string, actual any) *SnapshotResult {
	if globalManager == nil {
		return &SnapshotResult{
			Passed:  false,
			Message: "snapshot manager not initialized",
			Actual:  actual,
		}
	}
	return globalManager.Compare(testFile, requestName, snapshotName, actual)
}
