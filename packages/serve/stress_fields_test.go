package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The result DTO carries the stress run's assertion counts and the rate-miss
// warning, not only the transport totals.
func TestStressResultCarriesAssertionAndWarningFields(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer target.Close()

	dir := t.TempDir()
	file := "### chk\n# @name chk\nGET " + target.URL + "/x\n\n>>>\nexpect status 200\nexpect body.ok == true\n<<<\n"
	if err := os.WriteFile(filepath.Join(dir, "a.http"), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(WithWorkDir(dir), WithEnv("test"), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx, s.cancel = ctx, cancel

	// One slot at ~120ms per request cannot reach 40 req/s.
	result := startStressAndWait(t, s, `{"files":["a.http"],"duration":"1s","rate":40,"maxVUs":1}`)

	if result.AssertionChecks == 0 || result.AssertionFailures != result.AssertionChecks {
		t.Fatalf("assertion counts = %d failures of %d checks; want every checked response to fail", result.AssertionFailures, result.AssertionChecks)
	}
	if result.AssertionFailRate < 0.99 {
		t.Fatalf("assertionFailRate = %v", result.AssertionFailRate)
	}
	if result.Errors != 0 {
		t.Fatalf("a 200 with a failed assertion is not an error: errors = %d", result.Errors)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "below the 40 req/s target") {
		t.Fatalf("warnings = %v; want the rate-miss warning", result.Warnings)
	}
	if len(result.Breakdown) != 1 || result.Breakdown[0].AssertionFailures != result.AssertionFailures {
		t.Fatalf("breakdown = %+v", result.Breakdown)
	}
	b, _ := json.Marshal(result)
	for _, key := range []string{`"assertionChecks"`, `"assertionFailures"`, `"assertionFailRate"`, `"warnings"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("result JSON lacks %s: %s", key, b)
		}
	}
}

// Clean runs keep the old JSON shape: the new fields are omitted when empty.
func TestStressResultOmitsEmptyNewFields(t *testing.T) {
	b, _ := json.Marshal(StressResultDTO{})
	for _, key := range []string{"incomplete", "assertionChecks", "assertionFailures", "assertionFailRate", "warnings"} {
		if strings.Contains(string(b), key) {
			t.Errorf("empty result JSON contains %q: %s", key, b)
		}
	}
}
