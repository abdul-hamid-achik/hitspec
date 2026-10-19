package serve

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A busy port makes /mock/start and /record/start answer an error instead of
// "started" for a server nothing listens on.
func TestMockAndRecordStartReportBusyPort(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mock.http"), []byte("### ping\n# @name ping\nGET http://localhost/ping\n\n>>>\nexpect status 200\n<<<\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, base, _ := startSecured(t, WithWorkDir(dir))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s.ctx = ctx

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	busy := ln.Addr().(*net.TCPAddr).Port

	resp := postJ(t, base+"/api/v1/mock/start", `{"files":["mock.http"],"port":`+strconv.Itoa(busy)+`}`)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(readBody(t, resp), "cannot listen") {
		t.Fatalf("mock/start on a busy port = %d; want 409 with a listen error", resp.StatusCode)
	}
	s.mu.Lock()
	running := s.mockServer != nil
	s.mu.Unlock()
	if running {
		t.Fatal("mock server registered after a failed start")
	}

	resp = postJ(t, base+"/api/v1/record/start", `{"targetUrl":"http://127.0.0.1:1","port":`+strconv.Itoa(busy)+`}`)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(readBody(t, resp), "cannot listen") {
		t.Fatalf("record/start on a busy port = %d; want 409 with a listen error", resp.StatusCode)
	}
	s.mu.Lock()
	running = s.recorder != nil
	s.mu.Unlock()
	if running {
		t.Fatal("recorder registered after a failed start")
	}
}
