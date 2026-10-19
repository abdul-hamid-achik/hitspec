package clientmgr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Starting from a saved profile applies its duration and thresholds: a bare
// rps threshold means a minimum, like the CLI's --profile.
func TestStartStressFromProfileAppliesThresholds(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### ok\n# @name ok\nGET "+target.URL+"/x\n"); err != nil {
		t.Fatal(err)
	}
	if err := m.PutStressProfile(ctx, StressProfileReq{
		Name: "gate", Duration: "1s", Rate: 5,
		Thresholds: map[string]string{"rps": "100000", "p95": "10m"},
	}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"a.http"}, Profile: "gate"}); err != nil {
		t.Fatal(err)
	}
	var result *StressResultDTO
	waitFor(t, 15*time.Second, "stress result", func() bool {
		r, err := m.StressResult(ctx)
		result = r
		return err == nil
	})
	if time.Since(start) > 14*time.Second {
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

// Request values win over the profile's, like explicit CLI flags; the
// profile's thresholds still apply.
func TestStartStressExplicitValuesBeatProfile(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "a.http", "### ok\n# @name ok\nGET "+target.URL+"/x\n"); err != nil {
		t.Fatal(err)
	}
	if err := m.PutStressProfile(ctx, StressProfileReq{
		Name: "slow", Duration: "1m", Rate: 5,
		Thresholds: map[string]string{"errors": "<=0%"},
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"a.http"}, Profile: "slow", Duration: "1s", Rate: 5}); err != nil {
		t.Fatal(err)
	}
	var result *StressResultDTO
	waitFor(t, 15*time.Second, "stress result", func() bool {
		r, err := m.StressResult(ctx)
		result = r
		return err == nil
	})
	if time.Since(start) > 14*time.Second {
		t.Errorf("run took %v; the request's 1s duration did not beat the profile's 1m", time.Since(start))
	}
	if len(result.Thresholds) == 0 {
		t.Error("the profile's thresholds were dropped when the request set its own duration")
	}
}

func TestStartStressUnknownProfileFails(t *testing.T) {
	m := newTestManager(t)
	err := m.StartStress(context.Background(), StressStartReq{Files: []string{"a.http"}, Profile: "nope"})
	if err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("err = %v; want one naming the profile", err)
	}
}
