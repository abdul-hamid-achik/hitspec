package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeReport runs one report pass the way a watch re-run does: fresh
// formatter and destination, one result, flush, close.
func writeReport(t *testing.T, format, path, requestName string) {
	t.Helper()
	f, closeReport, err := openReport(format, path, 0, true, false)
	require.NoError(t, err)
	f.FormatResult(&runner.RunResult{
		File:    "x.http",
		Passed:  1,
		Results: []*runner.RequestResult{{Name: requestName, Passed: true}},
	})
	flushable, ok := f.(Flushable)
	require.True(t, ok)
	require.NoError(t, flushable.Flush(time.Millisecond))
	require.NoError(t, closeReport())
}

// TestOpenReport_TruncatesPerRun guards watch mode with --output-file: the file
// was opened once and every re-run appended another JSON document to it, which
// is invalid JSON. Each pass must leave only that pass's report.
func TestOpenReport_TruncatesPerRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")

	writeReport(t, "json", path, "first-run-request-with-a-long-name")
	writeReport(t, "json", path, "second")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc struct {
		Tests []struct {
			Name string `json:"name"`
		} `json:"tests"`
	}
	require.NoError(t, json.Unmarshal(data, &doc), "report must be exactly one valid JSON document, got: %s", data)
	require.Len(t, doc.Tests, 1)
	assert.Equal(t, "second", doc.Tests[0].Name)
}

// TestOpenReport_Destinations covers where the report goes for each mode.
func TestOpenReport_Destinations(t *testing.T) {
	// --quiet without --output-file must not fall through to stdout (the
	// formatter default), and the typed-nil writer pitfall must not reappear:
	// a plain run still gets a usable formatter.
	f, closeReport, err := openReport("console", "", 0, true, true)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.NoError(t, closeReport())

	f, closeReport, err = openReport("json", "", 0, true, false)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.NoError(t, closeReport())

	// An unwritable path is reported, not swallowed.
	_, _, err = openReport("json", filepath.Join(t.TempDir(), "missing-dir", "r.json"), 0, true, false)
	assert.ErrorContains(t, err, "cannot create output file")
}

func TestExitErrorFor_Precedence(t *testing.T) {
	code := func(err error) int {
		if err == nil {
			return 0
		}
		var coder ExitCoder
		require.True(t, errors.As(err, &coder))
		return coder.ExitCode()
	}

	assert.Equal(t, ExitSuccess, code(exitErrorFor(runTotals{Passed: 2, Matched: 2}, false)))
	assert.Equal(t, ExitTestFailure, code(exitErrorFor(runTotals{Failed: 1, Matched: 1}, true)))
	assert.Equal(t, ExitParseError, code(exitErrorFor(runTotals{BrokenFiles: 1}, false)))
	assert.Equal(t, ExitParseError, code(exitErrorFor(runTotals{BrokenFiles: 1, Failed: 3}, true)), "parse error beats test failure")
	assert.Equal(t, ExitParseError, code(exitErrorFor(runTotals{BrokenFiles: 1}, true)), "parse error beats no-match")
	assert.Equal(t, ExitUsageError, code(exitErrorFor(runTotals{Skipped: 4}, true)), "filters matching nothing")
	assert.Equal(t, ExitSuccess, code(exitErrorFor(runTotals{Skipped: 1, Matched: 1}, true)), "an @skip'd match is still a match")
}

func TestResolveEnvironment(t *testing.T) {
	cfg := &config.Config{DefaultEnvironment: "staging"}

	name, explicit := resolveEnvironment("prod", cfg)
	assert.Equal(t, "prod", name)
	assert.True(t, explicit)

	name, explicit = resolveEnvironment("", cfg)
	assert.Equal(t, "staging", name)
	assert.False(t, explicit)

	name, explicit = resolveEnvironment("", &config.Config{})
	assert.Equal(t, "dev", name)
	assert.False(t, explicit)

	name, _ = resolveEnvironment("", nil)
	assert.Equal(t, "dev", name)
}

func TestNoMatchMessage(t *testing.T) {
	assert.Equal(t, "", noMatchMessage("", nil))
	assert.Equal(t, `no requests matched --tags "smok"`, noMatchMessage("", []string{"smok"}))
	assert.Equal(t, `no requests matched --name "x" --tags "a,b"`, noMatchMessage("x", []string{"a", "b"}))
}
