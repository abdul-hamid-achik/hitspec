package clientmgr

import (
	"context"
	"fmt"
	"testing"
)

// A request's warnings are stored with the run and come back from GetRun.
func TestHistoryKeepsRequestWarnings(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	content := fmt.Sprintf("### warn\n# @name warn\nGET %s/w\nX-Token: {{nobody.token}}\n", srv.URL)
	if _, err := m.CreateFile(ctx, "w.http", content); err != nil {
		t.Fatal(err)
	}
	res, err := m.RunFile(ctx, RunReq{File: "w.http"})
	if err != nil || len(res.Results) != 1 || len(res.Results[0].Warnings) == 0 {
		t.Fatalf("run = %+v, %v; want a warning", res, err)
	}

	runs, err := m.ListRuns(ctx, 10, 0)
	if err != nil || len(runs.Runs) == 0 {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	run, err := m.GetRun(ctx, runs.Runs[0].ID)
	if err != nil || len(run.Results) != 1 {
		t.Fatalf("GetRun = %+v, %v", run, err)
	}
	got := run.Results[0].Warnings
	if len(got) != len(res.Results[0].Warnings) || got[0] != res.Results[0].Warnings[0] {
		t.Fatalf("stored warnings = %v, want %v", got, res.Results[0].Warnings)
	}
}
