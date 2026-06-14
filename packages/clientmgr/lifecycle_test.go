package clientmgr

import (
	"context"
	"net"
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
