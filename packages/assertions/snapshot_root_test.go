package assertions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/snapshot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With a file root (serve, the studio) a __snapshots__ directory that is a
// symlink out of the workspace must not be read or written.
func TestEvaluator_SnapshotDirConfinedToFileRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, snapshot.SnapshotDir)); err != nil {
		// Symlinks need privileges on Windows (developer mode or admin).
		t.Skipf("symlinks unavailable: %v", err)
	}

	eval := func(fileRoot string, update bool) *Result {
		opts := []EvaluatorOption{
			WithTestFile(filepath.Join(root, "test.http")),
			WithRequestName("getUser"),
			WithSnapshotManager(snapshot.NewManager(root, update)),
		}
		if fileRoot != "" {
			opts = append(opts, WithFileRoot(fileRoot))
		}
		e := NewEvaluatorWithBaseDir(createResponse(200, `{"id": 1}`, nil), "", opts...)
		return e.Evaluate(&parser.Assertion{Subject: "body", Operator: parser.OpSnapshot, Expected: "snap"})
	}

	res := eval(root, true)
	assert.False(t, res.Passed, "writing through the symlink must be refused")
	assert.Contains(t, res.Message, "outside the workspace")
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing was written outside the workspace")

	// Without a file root (hitspec run on the CLI) the symlink is followed, as before.
	res = eval("", true)
	assert.True(t, res.Passed, res.Message)
	entries, err = os.ReadDir(outside)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
}

// A snapshot inside the root works as before when a root is set.
func TestEvaluator_SnapshotInsideFileRoot(t *testing.T) {
	root := t.TempDir()
	for _, update := range []bool{true, false} {
		e := NewEvaluatorWithBaseDir(createResponse(200, `{"id": 1}`, nil), "",
			WithTestFile(filepath.Join(root, "test.http")), WithRequestName("r"),
			WithSnapshotManager(snapshot.NewManager(root, update)), WithFileRoot(root))
		res := e.Evaluate(&parser.Assertion{Subject: "body", Operator: parser.OpSnapshot, Expected: "snap"})
		assert.True(t, res.Passed, res.Message)
	}
}
