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

const stressProfileYAML = `stress:
  profiles:
    gate:
      duration: 1s
      rate: 5
      thresholds:
        rps: "100000"
        p95: "10m"
    slow:
      duration: 1m
      rate: 5
      thresholds:
        errors: "<=0%"
`

func stressProfileServer(t *testing.T, target string) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte(stressProfileYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.http"), []byte("### ok\n# @name ok\nGET "+target+"/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewServer(WithWorkDir(dir), WithEnv("test"), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s.ctx, s.cancel = ctx, cancel
	return s
}

// Starting from a saved profile applies its duration and thresholds: a bare
// rps threshold means a minimum, like the CLI's --profile.
func TestStressStartFromProfileAppliesThresholds(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	s := stressProfileServer(t, target.URL)

	start := time.Now()
	result := startStressAndWait(t, s, `{"files":["a.http"],"profile":"gate"}`)
	if time.Since(start) > 20*time.Second {
		t.Errorf("run took %v; the profile's 1s duration was not used", time.Since(start))
	}
	got := map[string]StressThresholdDTO{}
	for _, th := range result.Thresholds {
		got[th.Name] = th
	}
	if th, ok := got["min RPS"]; !ok || th.Passed || th.Expected != "> 100000" {
		t.Errorf("min RPS threshold = %+v (present %v); want a failing '> 100000'", th, ok)
	}
	if th, ok := got["p95"]; !ok || !th.Passed || th.Expected != "< 10m0s" {
		t.Errorf("p95 threshold = %+v (present %v); want a passing '< 10m0s'", th, ok)
	}
}

// Values in the request win over the profile's, like explicit CLI flags.
func TestStressStartExplicitValuesBeatProfile(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	s := stressProfileServer(t, target.URL)

	start := time.Now()
	result := startStressAndWait(t, s, `{"files":["a.http"],"profile":"slow","duration":"1s","rate":5}`)
	if time.Since(start) > 20*time.Second {
		t.Errorf("run took %v; the request's 1s duration did not beat the profile's 1m", time.Since(start))
	}
	if len(result.Thresholds) == 0 {
		t.Error("the profile's thresholds were dropped when the request set its own duration")
	}
}

func TestStressStartUnknownProfileIsBadRequest(t *testing.T) {
	s := stressProfileServer(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stress/start", strings.NewReader(`{"files":["a.http"],"profile":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleStressStart(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `nope`) {
		t.Fatalf("got %d %s; want 400 naming the profile", w.Code, w.Body.String())
	}
}

// The status reports the run's own duration (here the profile's), so a client
// can draw progress without guessing from its form.
func TestStressStatusReportsTheRunDuration(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	s := stressProfileServer(t, target.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stress/start", strings.NewReader(`{"files":["a.http"],"profile":"slow"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleStressStart(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	defer s.handleStressStop(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/stress/stop", nil))

	w = httptest.NewRecorder()
	s.handleStressStatus(w, httptest.NewRequest(http.MethodGet, "/api/v1/stress/status", nil))
	var st StressStatusDTO
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.Duration != 60 {
		t.Fatalf("status = %+v; want a running test with duration 60", st)
	}
}
