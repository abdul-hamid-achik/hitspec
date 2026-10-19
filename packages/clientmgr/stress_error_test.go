package clientmgr

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// A stress run that stops before producing numbers (here a setup request that
// cannot connect) reports why, instead of a clean result with zero requests.
func TestStressResultCarriesRunError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + l.Addr().String()
	_ = l.Close()

	ctx := context.Background()
	m := newTestManager(t)
	src := "### login\n# @name login\n# @stress.setup\nPOST " + closed + "/login\n\n### load\n# @name load\nGET " + closed + "/x\n"
	if _, err := m.CreateFile(ctx, "a.http", src); err != nil {
		t.Fatal(err)
	}
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := m.Subscribe(subCtx)
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"a.http"}, Duration: "1s", Rate: 5}); err != nil {
		t.Fatal(err)
	}
	var result *StressResultDTO
	waitFor(t, 15*time.Second, "stress result", func() bool {
		r, err := m.StressResult(ctx)
		result = r
		return err == nil && r != nil
	})
	if !strings.Contains(result.Error, `setup request "login" failed`) {
		t.Fatalf("result.Error = %q; want the setup failure", result.Error)
	}
	if result.Total != 0 {
		t.Fatalf("total = %d; no load should have run", result.Total)
	}
	// The completed stress_update event says so too (the TUI and the desktop
	// app only learn about the end of a run from it).
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			if sm, ok := ev.Payload.(StressMetrics); ok && sm.Completed {
				if sm.Error != result.Error {
					t.Fatalf("completed event error = %q, want %q", sm.Error, result.Error)
				}
				return
			}
		case <-deadline:
			t.Fatal("no completed stress_update event")
		}
	}
}
