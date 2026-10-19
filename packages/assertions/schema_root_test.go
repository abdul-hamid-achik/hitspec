package assertions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With a file root, a schema reached through a symlink that leaves the root is
// refused; without one (the CLI) it is read as before.
func TestEvaluator_Schema_FileRootFollowsSymlinks(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "s.json"), []byte(`{"type":"object"}`), 0o644))
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "s.json"), filepath.Join(root, "link.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "in.json"), []byte(`{"type":"object"}`), 0o644))

	check := func(schema string, opts ...EvaluatorOption) *Result {
		e := NewEvaluatorWithBaseDir(createResponse(200, `{}`, nil), root, opts...)
		return e.Evaluate(&parser.Assertion{Subject: "body", Operator: parser.OpSchema, Expected: schema})
	}

	assert.True(t, check("link.json").Passed, "no file root: the CLI keeps reading it")
	assert.True(t, check("in.json", WithFileRoot(root)).Passed)
	res := check("link.json", WithFileRoot(root))
	assert.False(t, res.Passed)
	assert.Contains(t, res.Message, "outside the workspace")
}
