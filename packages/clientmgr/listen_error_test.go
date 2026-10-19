package clientmgr

import (
	"context"
	"net"
	"strings"
	"testing"
)

// occupyPort holds a listening socket for the test and returns its port.
func occupyPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

// A busy port fails StartMock/StartRecord synchronously and leaves nothing
// registered as running (it used to report "started" while nothing listened).
func TestStartMockAndRecordReportBusyPort(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "mock.http", "### ping\n# @name ping\nGET http://localhost/ping\n\n>>>\nexpect status 200\n<<<\n"); err != nil {
		t.Fatal(err)
	}
	busy := occupyPort(t)

	status, err := m.StartMock(ctx, MockStartReq{Files: []string{"mock.http"}, Port: busy})
	if err == nil || !strings.Contains(err.Error(), "cannot listen") {
		t.Fatalf("StartMock on a busy port = %+v, %v; want a listen error", status, err)
	}
	if m.MockStatus(ctx).Running {
		t.Fatal("mock reported running after a failed start")
	}
	// The failed start does not wedge the manager: the same call works on a free port.
	if _, err := m.StartMock(ctx, MockStartReq{Files: []string{"mock.http"}, Port: freePort(t)}); err != nil {
		t.Fatalf("StartMock on a free port after a failure: %v", err)
	}

	target := okServer(t)
	err = m.StartRecord(ctx, RecordStartReq{TargetURL: target.URL, Port: busy})
	if err == nil || !strings.Contains(err.Error(), "cannot listen") {
		t.Fatalf("StartRecord on a busy port = %v; want a listen error", err)
	}
	if m.RecordStatus(ctx).Running {
		t.Fatal("recorder reported running after a failed start")
	}
	if err := m.StartRecord(ctx, RecordStartReq{TargetURL: target.URL, Port: freePort(t)}); err != nil {
		t.Fatalf("StartRecord on a free port after a failure: %v", err)
	}
}
