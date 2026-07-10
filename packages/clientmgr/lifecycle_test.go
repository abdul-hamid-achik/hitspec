package clientmgr

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// freePort grabs an ephemeral port and releases it, so a server can bind it next
// with very low chance of a collision.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestMockLifecycle(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "mock.http", "### ping\n# @name ping\nGET http://localhost/ping\n\n>>>\nexpect status 200\n<<<\n"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	if m.MockStatus(ctx).Running {
		t.Fatal("mock should not be running initially")
	}

	status, err := m.StartMock(ctx, MockStartReq{Files: []string{"mock.http"}, Port: freePort(t)})
	if err != nil {
		t.Fatalf("StartMock: %v", err)
	}
	if !status.Running {
		t.Fatalf("StartMock status = %+v, want running", status)
	}

	// Starting twice is rejected.
	if _, err := m.StartMock(ctx, MockStartReq{Files: []string{"mock.http"}, Port: freePort(t)}); err == nil {
		t.Fatal("second StartMock should error")
	}

	if err := m.StopMock(ctx); err != nil {
		t.Fatalf("StopMock: %v", err)
	}
	waitFor(t, 3*time.Second, "mock to stop", func() bool { return !m.MockStatus(ctx).Running })
}

func TestRecordLifecycle(t *testing.T) {
	ctx := context.Background()
	target := okServer(t)
	m := newTestManager(t)

	if err := m.StartRecord(ctx, RecordStartReq{TargetURL: target.URL, Port: freePort(t)}); err != nil {
		t.Fatalf("StartRecord: %v", err)
	}
	waitFor(t, 3*time.Second, "recorder to start", func() bool { return m.RecordStatus(ctx).Running })

	// TargetURL is required.
	m2 := newTestManager(t)
	if err := m2.StartRecord(ctx, RecordStartReq{}); err == nil {
		t.Fatal("StartRecord without TargetURL should error")
	}

	if _, err := m.ExportRecordings(ctx); err != nil {
		t.Fatalf("ExportRecordings: %v", err)
	}
	if err := m.ClearRecordings(ctx); err != nil {
		t.Fatalf("ClearRecordings: %v", err)
	}
	if err := m.StopRecord(ctx); err != nil {
		t.Fatalf("StopRecord: %v", err)
	}
	waitFor(t, 3*time.Second, "recorder to stop", func() bool { return !m.RecordStatus(ctx).Running })
}

func TestStressRunAndResult(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)

	if err := m.StartStress(ctx, StressStartReq{Files: []string{"api.http"}, Duration: "300ms", Rate: 5}); err != nil {
		t.Fatalf("StartStress: %v", err)
	}
	// Starting twice while running is rejected.
	if err := m.StartStress(ctx, StressStartReq{Files: []string{"api.http"}, Duration: "300ms", Rate: 5}); err == nil {
		t.Fatal("second StartStress should error")
	}

	waitFor(t, 5*time.Second, "stress run to finish", func() bool { return !m.StressStatus(ctx).Running })

	res, err := m.StressResult(ctx)
	if err != nil {
		t.Fatalf("StressResult: %v", err)
	}
	if res == nil || res.Total == 0 {
		t.Fatalf("stress result = %+v, want some requests", res)
	}
}

func TestStressStop(t *testing.T) {
	ctx := context.Background()
	srv := okServer(t)
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", srv.URL)

	if err := m.StartStress(ctx, StressStartReq{Files: []string{"api.http"}, Duration: "30s", Rate: 5}); err != nil {
		t.Fatalf("StartStress: %v", err)
	}
	if err := m.StopStress(ctx); err != nil {
		t.Fatalf("StopStress: %v", err)
	}
	waitFor(t, 5*time.Second, "stress to stop", func() bool { return !m.StressStatus(ctx).Running })
}

func TestStressProfilesCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if err := m.PutStressProfile(ctx, StressProfileReq{Name: "load", Duration: "30s", Rate: 10}); err != nil {
		t.Fatalf("PutStressProfile: %v", err)
	}
	profiles, err := m.ListStressProfiles(ctx)
	if err != nil {
		t.Fatalf("ListStressProfiles: %v", err)
	}
	found := false
	for _, p := range profiles {
		if p.Name == "load" {
			found = true
		}
	}
	if !found {
		t.Fatalf("profile 'load' not in %+v", profiles)
	}

	if err := m.DeleteStressProfile(ctx, "load"); err != nil {
		t.Fatalf("DeleteStressProfile: %v", err)
	}
	profiles, _ = m.ListStressProfiles(ctx)
	for _, p := range profiles {
		if p.Name == "load" {
			t.Fatal("profile 'load' should be deleted")
		}
	}
}

// TestStopRecordPreservesRecordings verifies that stopping the recording proxy
// keeps captured requests in memory so ExportRecordings/RecordStatus still
// return them afterwards. Only ClearRecordings should drop them. This is a
// regression test for the bug where stop destroyed captures before export.
func TestStopRecordPreservesRecordings(t *testing.T) {
	ctx := context.Background()
	target := okServer(t)
	m := newTestManager(t)
	port := freePort(t)

	if err := m.StartRecord(ctx, RecordStartReq{TargetURL: target.URL, Port: port}); err != nil {
		t.Fatalf("StartRecord: %v", err)
	}
	waitFor(t, 3*time.Second, "recorder to start", func() bool { return m.RecordStatus(ctx).Running })

	// Drive at least one request through the proxy so a capture exists.
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d/ping", port)
	waitFor(t, 3*time.Second, "proxy to capture a request", func() bool {
		resp, err := http.Get(proxyURL)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return m.RecordStatus(ctx).Count >= 1
	})

	countBefore := m.RecordStatus(ctx).Count
	if countBefore < 1 {
		t.Fatalf("expected at least 1 recording before stop, got %d", countBefore)
	}

	// Stop the proxy — recordings must survive this.
	if err := m.StopRecord(ctx); err != nil {
		t.Fatalf("StopRecord: %v", err)
	}
	waitFor(t, 3*time.Second, "recorder to stop", func() bool { return !m.RecordStatus(ctx).Running })

	// Status must still report the captured recordings (now not running).
	status := m.RecordStatus(ctx)
	if status.Running {
		t.Fatal("recorder should not be running after stop")
	}
	if status.Count < countBefore {
		t.Fatalf("recordings lost after stop: count %d, want >= %d", status.Count, countBefore)
	}
	if len(status.Recordings) < countBefore {
		t.Fatalf("recordings list lost after stop: len %d, want >= %d", len(status.Recordings), countBefore)
	}

	// Export must still work and contain the captured path.
	out, err := m.ExportRecordings(ctx)
	if err != nil {
		t.Fatalf("ExportRecordings after stop: %v", err)
	}
	if !strings.Contains(out, "/ping") {
		t.Fatalf("export after stop missing captured request, got:\n%s", out)
	}

	// Only an explicit clear should drop the recordings.
	if err := m.ClearRecordings(ctx); err != nil {
		t.Fatalf("ClearRecordings after stop: %v", err)
	}
	if got := m.RecordStatus(ctx).Count; got != 0 {
		t.Fatalf("recordings should be empty after clear, got %d", got)
	}
	if out, err := m.ExportRecordings(ctx); err != nil {
		t.Fatalf("ExportRecordings after clear errored: %v", err)
	} else if strings.Contains(out, "/ping") {
		t.Fatalf("export after clear should not contain captured request, got:\n%s", out)
	}
}
