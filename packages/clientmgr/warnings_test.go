package clientmgr

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Request warnings reach the result DTO and the request_progress event.
func TestWarningsReachDTOAndProgress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := okServer(t)
	m := newTestManager(t)
	content := fmt.Sprintf("### warn\n# @name warn\nGET %s/w\nX-Token: {{nobody.token}}\n", srv.URL)
	if _, err := m.CreateFile(ctx, "w.http", content); err != nil {
		t.Fatal(err)
	}
	events := m.Subscribe(ctx)

	res, err := m.RunFile(ctx, RunReq{File: "w.http"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 1 || len(res.Results[0].Warnings) == 0 {
		t.Fatalf("result DTO has no warnings: %+v", res.Results)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			p, ok := ev.Payload.(RequestProgress)
			if !ok || p.Status != "completed" {
				continue
			}
			if len(p.Warnings) == 0 || p.Warnings[0] != res.Results[0].Warnings[0] {
				t.Fatalf("progress warnings = %v, want %v", p.Warnings, res.Results[0].Warnings)
			}
			return
		case <-deadline:
			t.Fatal("no completed request_progress event")
		}
	}
}
