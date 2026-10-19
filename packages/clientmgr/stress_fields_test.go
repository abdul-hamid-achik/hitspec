package clientmgr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### chk\n# @name chk\nGET "+target.URL+"/x\n\n>>>\nexpect status 200\nexpect body.ok == true\n<<<\n"); err != nil {
		t.Fatal(err)
	}
	// One slot at ~120ms per request cannot reach 40 req/s.
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"a.http"}, Duration: "1s", Rate: 40, MaxVUs: 1}); err != nil {
		t.Fatal(err)
	}
	var result *StressResultDTO
	waitFor(t, 15*time.Second, "stress result", func() bool {
		r, err := m.StressResult(ctx)
		result = r
		return err == nil
	})

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
