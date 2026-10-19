package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

// ---- request_progress is filtered by ExecID -------------------------------

func progressEvent(id, name string, idx, total int) clientmgr.Event {
	return clientmgr.Event{Type: "request_progress", Payload: clientmgr.RequestProgress{
		ExecID: id, RequestName: name, Index: idx, Total: total,
	}}
}

func TestProgressFromAnotherExecutionIsIgnored(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.running, m.activeExec = true, "mine"

	m.handleEvent(progressEvent("old-cancelled", "stale", 2, 5))
	if m.progress.Total != 0 {
		t.Fatalf("an event from another execution updated the view: %+v", m.progress)
	}
	m.handleEvent(progressEvent("mine", "current", 1, 3))
	if m.progress.RequestName != "current" || m.progress.Total != 3 {
		t.Fatalf("the active execution's event was dropped: %+v", m.progress)
	}
	m.handleEvent(progressEvent("old-cancelled", "stale", 5, 5))
	if m.progress.RequestName != "current" || m.progress.Index != 1 {
		t.Fatalf("a late event from another execution overwrote the view: %+v", m.progress)
	}
	// An event with no ID cannot be attributed to this run either.
	m.handleEvent(progressEvent("", "anon", 4, 4))
	if m.progress.RequestName != "current" {
		t.Fatalf("an unattributed event overwrote the view: %+v", m.progress)
	}
}

func TestEachRunGetsItsOwnExecID(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if _, _, ok := m.beginRun(); !ok {
		t.Fatal("beginRun")
	}
	first := m.activeExec
	if first == "" {
		t.Fatal("a run in flight has an exec id")
	}
	m.endRun()
	if m.activeExec != "" {
		t.Fatalf("endRun left activeExec = %q", m.activeExec)
	}
	m.beginRun()
	if m.activeExec == "" || m.activeExec == first {
		t.Fatalf("second run reused the exec id: %q then %q", first, m.activeExec)
	}
}

// The context beginRun hands out makes the manager publish under the model's
// ID, and a concurrent execution started without it is told apart.
func TestRunEventsCarryTheStudioExecID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "ok") }))
	defer srv.Close()
	mgr := newTestManager(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	src := "### One\n# @name one\nGET " + srv.URL + "/1\n\n### Two\n# @name two\nGET " + srv.URL + "/2\n"
	if _, err := mgr.CreateFile(ctx, "r.http", src); err != nil {
		t.Fatal(err)
	}
	events := mgr.Subscribe(ctx)

	m := newModel(ctx, mgr, Options{})
	runCtx, _, ok := m.beginRun()
	if !ok {
		t.Fatal("beginRun")
	}
	// Some other execution (no studio ID) runs first, then ours.
	if _, err := mgr.RunFile(ctx, clientmgr.RunReq{File: "r.http"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.RunFile(runCtx, clientmgr.RunReq{File: "r.http"}); err != nil {
		t.Fatal(err)
	}

	mine, other := 0, 0
	deadline := time.After(5 * time.Second)
	for done := false; !done; {
		select {
		case ev := <-events:
			if p, ok := ev.Payload.(clientmgr.RequestProgress); ok {
				if p.ExecID == m.activeExec {
					mine++
				} else {
					other++
				}
			}
			m.handleEvent(ev)
			if ev.Type == "execution_complete" && mine > 0 {
				done = true
			}
		case <-deadline:
			t.Fatalf("timed out; mine=%d other=%d", mine, other)
		}
	}
	if mine == 0 || other == 0 {
		t.Fatalf("expected events from both executions, mine=%d other=%d", mine, other)
	}
	if m.progress.ExecID != m.activeExec {
		t.Fatalf("the view shows another execution's progress: %+v", m.progress)
	}
	if strings.TrimSpace(m.progress.RequestName) == "" {
		t.Fatalf("no progress recorded: %+v", m.progress)
	}
}

// ---- stress progress uses the run's own duration --------------------------

func stressModel(t *testing.T) model {
	t.Helper()
	m := goldenModel(t, 100, 30)
	m.setScreen(screenStress)
	return m
}

var stressTestStats = clientmgr.StressStatsDTO{Total: 100, Success: 100, RPS: 3.3, ActiveVUs: 1}

// The form still says the default 30s, but the run (say, started from a profile
// or by another client) lasts 100s: the bar follows the run.
func TestStressProgressIgnoresTheFormDuration(t *testing.T) {
	m := stressModel(t)
	m.formInputs[0].SetValue("30s")
	stats := stressTestStats
	m.stress = clientmgr.StressStatusDTO{Running: true, Elapsed: 25, Duration: 100, Stats: &stats}
	out := plain(m.stressContent())
	if !strings.Contains(out, "25%") {
		t.Fatalf("25s of a 100s run should read 25%%, got:\n%s", out)
	}
}

// With no known duration there is no bar and no percentage, only elapsed time.
func TestStressProgressWithoutDurationShowsElapsedOnly(t *testing.T) {
	m := stressModel(t)
	m.formInputs[0].SetValue("30s") // must not be used as a fallback
	stats := stressTestStats
	m.stress = clientmgr.StressStatusDTO{Running: true, Elapsed: 12.5, Stats: &stats}
	out := plain(m.stressContent())
	if !strings.Contains(out, "Elapsed: 12.5s") {
		t.Fatalf("elapsed time missing:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "▌") || strings.Contains(line, "░") ||
			(strings.Contains(line, "%") && !strings.HasPrefix(line, "Error rate")) {
			t.Fatalf("a progress bar or percentage without a known duration: %q\n%s", line, out)
		}
	}
	if !strings.Contains(out, "Requests: 100") {
		t.Fatalf("metrics should still render:\n%s", out)
	}
}

// stress_update events carry the duration into the view.
func TestStressUpdateEventCarriesDuration(t *testing.T) {
	m := stressModel(t)
	m.handleEvent(clientmgr.Event{Type: "stress_update", Payload: clientmgr.StressMetrics{
		Running: true, Elapsed: 10, Duration: 40, Stats: stressTestStats,
	}})
	if m.stress.Duration != 40 {
		t.Fatalf("duration not kept: %+v", m.stress)
	}
	if out := plain(m.stressContent()); !strings.Contains(out, "25%") {
		t.Fatalf("10s of a 40s run should read 25%%:\n%s", out)
	}
}

// End to end: the manager reports the duration it was started with, in the
// status and in the events.
func TestManagerReportsStressDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "ok") }))
	defer srv.Close()
	mgr := newTestManager(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	if _, err := mgr.CreateFile(ctx, "s.http", "### S\nGET "+srv.URL+"/s\n"); err != nil {
		t.Fatal(err)
	}
	events := mgr.Subscribe(ctx)
	if err := mgr.StartStress(ctx, clientmgr.StressStartReq{Files: []string{"s.http"}, Duration: "2s", Rate: 5}); err != nil {
		t.Fatal(err)
	}
	if st := mgr.StressStatus(ctx); st.Running && st.Duration != 2 {
		t.Fatalf("status duration = %v, want 2", st.Duration)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev := <-events:
			if sm, ok := ev.Payload.(clientmgr.StressMetrics); ok {
				if sm.Duration != 2 {
					t.Fatalf("event duration = %v, want 2", sm.Duration)
				}
				if sm.Completed {
					return
				}
			}
		case <-deadline:
			t.Fatal("stress run did not complete")
		}
	}
}

// newTestManagerDir is newTestManager that also returns the workspace directory,
// for tests that change files behind the manager's back.
func newTestManagerDir(t *testing.T) (*clientmgr.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	mgr := clientmgr.New(
		clientmgr.WithWorkDir(dir),
		clientmgr.WithWatch(false),
		clientmgr.WithHistoryDBPath(filepath.Join(dir, "history.db")),
	)
	mgr.Start(context.Background())
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr, dir
}

// ---- a file deleted outside studio keeps the unsaved edits ----------------

func filesWithout(t *testing.T, mgr *clientmgr.Manager) filesLoadedMsg {
	t.Helper()
	files, err := mgr.ListFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return filesLoadedMsg{files: files}
}

func toastTexts(m model) string {
	var parts []string
	for _, it := range m.toasts.items {
		parts = append(parts, it.text)
	}
	return strings.Join(parts, "\n")
}

func TestDeletedOnDiskKeepsUnsavedEdits(t *testing.T) {
	mgr, dir := newTestManagerDir(t)
	m := openModel(t, mgr, 120, 40, map[string]string{
		"a.http": "### A\n# @name A\nGET https://example.com/a\n",
		"b.http": "### B\n# @name B\nGET https://example.com/b\n",
	}, "a.http")
	edited := m.raw + "# my precious edit\n"
	m.source.SetValue(edited)
	m.dirty = true
	if err := os.Remove(filepath.Join(dir, "a.http")); err != nil {
		t.Fatal(err)
	}

	next, _ := m.Update(filesWithout(t, mgr))
	m = next.(model)
	if m.selected != "a.http" {
		t.Fatalf("selection dropped: %q", m.selected)
	}
	if m.source.Value() != edited || !m.dirty {
		t.Fatalf("the unsaved buffer was cleared: dirty=%v value=%q", m.dirty, m.source.Value())
	}
	if !m.deletedOnDisk {
		t.Fatal("the deletion should be flagged")
	}
	if got := toastTexts(m); !strings.Contains(got, "deleted on disk") {
		t.Fatalf("want a deleted-on-disk warning toast, got %q", got)
	}
	if !strings.Contains(plain(m.topbar()), "deleted on disk") {
		t.Fatalf("the top bar should say so: %q", plain(m.topbar()))
	}

	// The next refresh (another file event) must not warn again.
	n := len(m.toasts.items)
	next, _ = m.Update(filesWithout(t, mgr))
	m = next.(model)
	if len(m.toasts.items) != n {
		t.Fatalf("warned twice: %q", toastTexts(m))
	}
}

// ctrl+s recreates the file with the buffer.
func TestSaveRecreatesAFileDeletedOnDisk(t *testing.T) {
	mgr, dir := newTestManagerDir(t)
	m := openModel(t, mgr, 120, 40, map[string]string{
		"a.http": "### A\n# @name A\nGET https://example.com/a\n",
		"b.http": "### B\n# @name B\nGET https://example.com/b\n",
	}, "a.http")
	edited := "### A2\n# @name A2\nGET https://example.com/a2\n"
	m.source.SetValue(edited)
	m.dirty = true
	path := filepath.Join(dir, "a.http")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(filesWithout(t, mgr))
	m = next.(model)

	m, cmd := press(m, keyCtrlS)
	if cmd == nil {
		t.Fatal("ctrl+s returned no command")
	}
	msg, ok := cmd().(fileLoadedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("recreate failed: %#v", msg)
	}
	disk, err := os.ReadFile(path)
	if err != nil || string(disk) != edited {
		t.Fatalf("file not recreated with the buffer: %q %v", disk, err)
	}
	next, _ = m.Update(msg)
	m = next.(model)
	if m.dirty || m.deletedOnDisk || m.selected != "a.http" {
		t.Fatalf("state after recreate: dirty=%v deleted=%v selected=%q", m.dirty, m.deletedOnDisk, m.selected)
	}
}

// An emptied buffer is saved as empty, not replaced by the new-file template.
func TestSaveRecreatesWithEmptyBuffer(t *testing.T) {
	mgr, dir := newTestManagerDir(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", "### A\nGET https://example.com\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "a.http")); err != nil {
		t.Fatal(err)
	}
	recreateFile(ctx, mgr, "a.http", "")
	disk, err := os.ReadFile(filepath.Join(dir, "a.http"))
	if err != nil || len(disk) != 0 {
		t.Fatalf("empty buffer should recreate an empty file, got %q %v", disk, err)
	}
}

// A buffer that does not parse still reaches the disk and says so.
func TestSaveRecreatesUnparseableBuffer(t *testing.T) {
	mgr, dir := newTestManagerDir(t)
	ctx := context.Background()
	bad := "### A\nGET https://example.com\n\n>>>\nexpect bogus operator\n<<<\n"
	msg := recreateFile(ctx, mgr, "gone.http", bad)
	if _, ok := msg.(fileSavedUnparsedMsg); !ok {
		t.Fatalf("want fileSavedUnparsedMsg, got %#v", msg)
	}
	if disk, _ := os.ReadFile(filepath.Join(dir, "gone.http")); string(disk) != bad {
		t.Fatalf("unparseable buffer not written: %q", disk)
	}
}

// Clean buffers keep the old behaviour: the stale selection is dropped.
func TestDeletedOnDiskWithCleanBufferStillClears(t *testing.T) {
	mgr, dir := newTestManagerDir(t)
	m := openModel(t, mgr, 120, 40, map[string]string{
		"a.http": "### A\nGET https://example.com/a\n",
		"b.http": "### B\nGET https://example.com/b\n",
	}, "a.http")
	if err := os.Remove(filepath.Join(dir, "a.http")); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(filesWithout(t, mgr))
	m = next.(model)
	if m.selected == "a.http" || m.deletedOnDisk {
		t.Fatalf("clean buffer: selection kept (%q) or flagged", m.selected)
	}
	if strings.Contains(toastTexts(m), "deleted on disk") {
		t.Fatalf("unexpected warning: %q", toastTexts(m))
	}
}

// Deleting the open file from studio (D, y) with unsaved edits is the user's own
// confirmed action, not an outside deletion.
func TestDeletingFromStudioWithEditsIsNotAnOutsideDeletion(t *testing.T) {
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, map[string]string{
		"a.http": "### A\nGET https://example.com/a\n",
		"b.http": "### B\nGET https://example.com/b\n",
	}, "a.http")
	m.source.SetValue(m.raw + "# edit\n")
	m.dirty = true

	m.deleteCmd()
	if m.confirm == nil {
		t.Fatal("expected the delete confirmation")
	}
	m, cmd := press(m, rk('y'))
	for i := 0; i < 4 && cmd != nil; i++ { // proceedMsg -> delete -> simpleMsg
		next, c := m.Update(cmd())
		m, cmd = next.(model), c
		if _, ok := toastSeen(m); ok {
			break
		}
	}
	// The delete's own file event refreshes the list, in whichever order.
	next, _ := m.Update(filesWithout(t, mgr))
	m = next.(model)
	if m.selected == "a.http" || m.deletedOnDisk {
		t.Fatalf("selection kept after deleting from studio: %q", m.selected)
	}
	if strings.Contains(toastTexts(m), "deleted on disk") {
		t.Fatalf("own delete reported as an outside deletion: %q", toastTexts(m))
	}
}

func toastSeen(m model) (string, bool) {
	s := toastTexts(m)
	return s, strings.Contains(s, "deleted on disk")
}

// Renaming from studio emits a "deleted" event for the old path; with unsaved
// edits (already confirmed as discarded) that must not read as an outside delete
// even when the refreshed list lands before the rename's own result.
func TestRenamingFromStudioWithEditsIsNotAnOutsideDeletion(t *testing.T) {
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, map[string]string{
		"a.http": "### A\nGET https://example.com/a\n",
		"b.http": "### B\nGET https://example.com/b\n",
	}, "a.http")
	m.source.SetValue(m.raw + "# edit\n")
	m.dirty = true

	m.executeCommand("rename-file")
	next, _ := m.Update(m.prompt.onSubmit("c.http")())
	m = next.(model)
	if m.confirm == nil {
		t.Fatal("expected the discard confirmation")
	}
	m, cmd := press(m, rk('y'))
	next, rename := m.Update(cmd()) // proceedMsg: the rename starts
	m = next.(model)
	if m.removing != "a.http" {
		t.Fatalf("the rename should mark a.http as being removed, got %q", m.removing)
	}
	msg := rename() // renames on disk
	// The list refresh from the rename's file events arrives first.
	next, _ = m.Update(filesWithout(t, mgr))
	m = next.(model)
	if strings.Contains(toastTexts(m), "deleted on disk") || m.deletedOnDisk {
		t.Fatalf("own rename reported as an outside deletion: %q", toastTexts(m))
	}
	next, _ = m.Update(msg)
	m = next.(model)
	if m.selected != "c.http" || m.removing != "" || m.dirty {
		t.Fatalf("after rename: selected=%q removing=%q dirty=%v", m.selected, m.removing, m.dirty)
	}
}
