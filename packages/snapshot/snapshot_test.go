package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Compare_NewSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")

	manager := NewManager(tmpDir, true) // Update mode enabled

	result := manager.Compare(testFile, "getUser", "", map[string]any{
		"id":   1,
		"name": "John",
	})

	if !result.Passed {
		t.Errorf("expected passed to be true, got false: %s", result.Message)
	}
	if !result.IsNew {
		t.Error("expected IsNew to be true")
	}

	// Verify snapshot file was created
	snapshotPath := filepath.Join(tmpDir, SnapshotDir, "test.snap.json")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Error("expected snapshot file to be created")
	}
}

func TestManager_Compare_ExistingSnapshot_Match(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")

	manager := NewManager(tmpDir, true)

	// Create initial snapshot
	data := map[string]any{"id": 1, "name": "John"}
	result := manager.Compare(testFile, "getUser", "", data)
	if !result.Passed || !result.IsNew {
		t.Fatal("failed to create initial snapshot")
	}

	// Compare with same data
	manager2 := NewManager(tmpDir, false) // Update mode disabled
	result = manager2.Compare(testFile, "getUser", "", data)

	if !result.Passed {
		t.Errorf("expected match, got: %s", result.Message)
	}
}

func TestManager_Compare_ExistingSnapshot_Mismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")

	manager := NewManager(tmpDir, true)

	// Create initial snapshot
	result := manager.Compare(testFile, "getUser", "", map[string]any{
		"id":   1,
		"name": "John",
	})
	if !result.Passed || !result.IsNew {
		t.Fatal("failed to create initial snapshot")
	}

	// Compare with different data
	manager2 := NewManager(tmpDir, false) // Update mode disabled
	result = manager2.Compare(testFile, "getUser", "", map[string]any{
		"id":   1,
		"name": "Jane", // Different
	})

	if result.Passed {
		t.Error("expected mismatch, got passed")
	}
	if result.Message != "snapshot mismatch" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestManager_Compare_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")

	manager := NewManager(tmpDir, true)

	// Create initial snapshot
	result := manager.Compare(testFile, "getUser", "", map[string]any{"name": "John"})
	if !result.Passed || !result.IsNew {
		t.Fatal("failed to create initial snapshot")
	}

	// Update with different data
	result = manager.Compare(testFile, "getUser", "", map[string]any{"name": "Jane"})

	if !result.Passed {
		t.Errorf("expected passed, got: %s", result.Message)
	}
	if !result.WasUpdated {
		t.Error("expected WasUpdated to be true")
	}
}

func TestManager_Compare_NoSnapshotNoUpdateMode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")

	manager := NewManager(tmpDir, false) // Update mode disabled

	result := manager.Compare(testFile, "getUser", "", map[string]any{"name": "John"})

	if result.Passed {
		t.Error("expected failure when no snapshot exists and update mode disabled")
	}
}

func TestManager_Compare_WithSnapshotName(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.http")

	manager := NewManager(tmpDir, true)

	// Create snapshot with specific name
	result := manager.Compare(testFile, "getUser", "userResponse", map[string]any{"id": 1})
	if !result.Passed || !result.IsNew {
		t.Fatal("failed to create initial snapshot")
	}

	// Create another snapshot with different name
	result = manager.Compare(testFile, "getUser", "userList", []any{1, 2, 3})
	if !result.Passed || !result.IsNew {
		t.Fatal("failed to create second snapshot")
	}

	// Verify both snapshots exist separately
	manager2 := NewManager(tmpDir, false)

	result = manager2.Compare(testFile, "getUser", "userResponse", map[string]any{"id": 1})
	if !result.Passed {
		t.Errorf("first snapshot mismatch: %s", result.Message)
	}

	result = manager2.Compare(testFile, "getUser", "userList", []any{1, 2, 3})
	if !result.Passed {
		t.Errorf("second snapshot mismatch: %s", result.Message)
	}
}

func TestGenerateKey(t *testing.T) {
	manager := NewManager(".", false)

	tests := []struct {
		requestName  string
		snapshotName string
		expectPrefix string
	}{
		{"getUser", "response", "getUser::response"},
		{"getUser", "", "getUser"},
		{"", "response", "::response"},
		{"", "", "anon_"}, // Should start with anon_
	}

	for _, tt := range tests {
		key := manager.generateKey(tt.requestName, tt.snapshotName, nil)
		if tt.expectPrefix == "anon_" {
			if len(key) < 5 || key[:5] != "anon_" {
				t.Errorf("expected key starting with 'anon_', got %q", key)
			}
		} else if key != tt.expectPrefix {
			t.Errorf("generateKey(%q, %q): got %q, expected %q", tt.requestName, tt.snapshotName, key, tt.expectPrefix)
		}
	}
}
