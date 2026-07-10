package assertions

import (
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/http"
	"github.com/abdul-hamid-achik/hitspec/packages/snapshot"
	"github.com/stretchr/testify/assert"
)

// TestEvaluator_NumericCoercion covers the numeric-coercion path: a JSON number
// (float64), an int expected, and a numeric string must compare equal/by-order
// regardless of the underlying Go type.
func TestEvaluator_NumericCoercion(t *testing.T) {
	bodyResp := createResponse(200, `{"count": 5, "score": 4.5}`, nil)
	be := NewEvaluator(bodyResp)
	assert.True(t, be.Evaluate(&parser.Assertion{Subject: "body.count", Operator: parser.OpEquals, Expected: 5}).Passed, "float body == int expected")
	assert.True(t, be.Evaluate(&parser.Assertion{Subject: "body.count", Operator: parser.OpGreaterThan, Expected: 4}).Passed, "float body > int expected")
	assert.True(t, be.Evaluate(&parser.Assertion{Subject: "body.score", Operator: parser.OpLessThan, Expected: "5"}).Passed, "float body < numeric-string expected")
	assert.False(t, be.Evaluate(&parser.Assertion{Subject: "body.count", Operator: parser.OpEquals, Expected: 6}).Passed, "mismatch should fail")
	assert.False(t, be.Evaluate(&parser.Assertion{Subject: "body.count", Operator: parser.OpGreaterThan, Expected: "notanumber"}).Passed, "non-numeric expected should fail")
}

// TestEvaluator_Snapshot covers the snapshot operator end-to-end: first run
// creates the baseline, a second identical run passes, a changed body fails.
func TestEvaluator_Snapshot(t *testing.T) {
	dir := t.TempDir()
	defer snapshot.SetGlobalManager(nil)

	mkResp := func(body string) *http.Response { return createResponse(200, body, nil) }
	opts := func() []EvaluatorOption {
		return []EvaluatorOption{WithTestFile(filepath.Join(dir, "test.http")), WithRequestName("getUser")}
	}

	// Update mode: the first evaluation creates the baseline snapshot and passes.
	snapshot.SetGlobalManager(snapshot.NewManager(dir, true))
	e := NewEvaluatorWithBaseDir(mkResp(`{"id": 1, "name": "alice"}`), "", opts()...)
	r1 := e.Evaluate(&parser.Assertion{Subject: "body", Operator: parser.OpSnapshot, Expected: "getUserResponse"})
	assert.True(t, r1.Passed, "first snapshot should pass (baseline created): %s", r1.Message)
	// Check mode: identical body matches the baseline; a changed body fails.
	snapshot.SetGlobalManager(snapshot.NewManager(dir, false))
	e2 := NewEvaluatorWithBaseDir(mkResp(`{"id": 1, "name": "alice"}`), "", opts()...)
	r2 := e2.Evaluate(&parser.Assertion{Subject: "body", Operator: parser.OpSnapshot, Expected: "getUserResponse"})
	assert.True(t, r2.Passed, "unchanged body should match snapshot: %s", r2.Message)

	// A changed body fails.
	e3 := NewEvaluatorWithBaseDir(mkResp(`{"id": 1, "name": "bob"}`), "", opts()...)
	r3 := e3.Evaluate(&parser.Assertion{Subject: "body", Operator: parser.OpSnapshot, Expected: "getUserResponse"})
	assert.False(t, r3.Passed, "changed body should fail the snapshot")
}
