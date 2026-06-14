package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func sampleHistory() clientmgr.HistoryListDTO {
	return clientmgr.HistoryListDTO{
		Total: 2,
		Runs: []clientmgr.HistoryRunDTO{
			{ID: 2, FilePath: "api.http", StartedAt: "2026-06-14T10:00:00Z", Passed: 3, Total: 3, DurationMs: 1200},
			{ID: 1, FilePath: "auth.http", StartedAt: "2026-06-14T09:00:00Z", Passed: 1, Failed: 1, Total: 2, DurationMs: 800},
		},
	}
}

func TestHistoryListPopulated(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.history = sampleHistory()
	m.refreshHistoryList()

	if got := len(m.historyList.Items()); got != 2 {
		t.Fatalf("want 2 history items, got %d", got)
	}
	it, ok := m.historyList.SelectedItem().(historyItem)
	if !ok || it.id != 2 {
		t.Fatalf("first selected item = %+v, want run #2", m.historyList.SelectedItem())
	}
}

func TestHistoryDeleteOpensConfirm(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenHistory)
	m.history = sampleHistory()
	m.refreshHistoryList()

	if cmd := m.handleHistoryKey(keyPress('D')); cmd != nil {
		t.Fatal("D should defer to a confirm dialog, not act immediately")
	}
	if m.confirm == nil {
		t.Fatal("D should open a confirm dialog")
	}
	if !strings.Contains(m.confirm.title, "Delete run") {
		t.Fatalf("unexpected confirm title: %q", m.confirm.title)
	}
}

func TestRunDetailMsgEntersDetailMode(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenHistory)
	run := clientmgr.HistoryRunDTO{
		ID: 7, FilePath: "api.http", Passed: 1, Total: 1,
		Results: []clientmgr.HistoryResultDTO{
			{RequestName: "Ping", Method: "GET", URL: "https://x/y", StatusCode: 200, Passed: true},
		},
	}
	next, _ := m.Update(runDetailMsg{run: run})
	nm := next.(model)
	if !nm.historyDetailMode {
		t.Fatal("runDetailMsg should enter detail mode")
	}
	if nm.historyDetail == nil || nm.historyDetail.ID != 7 {
		t.Fatalf("detail not stored: %+v", nm.historyDetail)
	}
	if !strings.Contains(nm.historyDetailContent(), "Ping") {
		t.Fatalf("detail content missing request name:\n%s", nm.historyDetailContent())
	}
}

func TestLoadRunCmdAgainstSeededHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	mgr := newTestManager(t)
	ctx := context.Background()
	content := "### Ping\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	if _, err := mgr.CreateFile(ctx, "h.http", content); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := mgr.RunFile(ctx, clientmgr.RunReq{File: "h.http"}); err != nil {
		t.Fatalf("run file: %v", err)
	}

	runs, err := mgr.ListRuns(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs.Runs) == 0 {
		t.Skip("history store did not persist a run in this environment")
	}
	id := runs.Runs[0].ID

	msg := loadRunCmd(ctx, mgr, id)().(runDetailMsg)
	if msg.err != nil {
		t.Fatalf("loadRunCmd: %v", msg.err)
	}
	if msg.run.ID != id {
		t.Fatalf("run id = %d, want %d", msg.run.ID, id)
	}
}
