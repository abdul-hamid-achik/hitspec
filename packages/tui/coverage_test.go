package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// drainBatch runs a (possibly batched) command and returns the messages it
// produced, so tests can inspect the result of tea.Batch(...) commands.
func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			if c != nil {
				out = append(out, c())
			}
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestSaveCmd(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "s.http", "### A\nGET https://x\n"); err != nil {
		t.Fatalf("create: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "s.http"
	m.source.SetValue("### B\nGET https://y\n")

	msg := m.saveCmd()().(fileLoadedMsg)
	if msg.err != nil {
		t.Fatalf("saveCmd: %v", msg.err)
	}
	raw, _ := mgr.ReadFile(ctx, "s.http")
	if !strings.Contains(raw, "https://y") {
		t.Fatalf("save did not persist new content: %q", raw)
	}
}

func TestSaveCmdNoSelection(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if m.saveCmd() != nil {
		t.Fatal("saveCmd with no selection should return nil")
	}
}

func TestRunFileCmdExecutes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "r.http", "### Ping\nGET "+srv.URL+"\n\n>>>\nexpect status 200\n<<<\n"); err != nil {
		t.Fatalf("create: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "r.http"

	var found bool
	for _, msg := range drainBatch(m.runFileCmd()) {
		if rd, ok := msg.(runDoneMsg); ok && rd.err == nil && rd.result != nil && rd.result.Passed == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("runFileCmd did not produce a passing runDoneMsg")
	}
}

func TestRunRequestCmdNoSelection(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if m.runRequestCmd() != nil {
		t.Fatal("runRequestCmd with no selection should return nil")
	}
}

func TestQuickCreateCmd(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	msg := m.quickCreateCmd()().(fileLoadedMsg)
	if msg.err != nil {
		t.Fatalf("quickCreateCmd: %v", msg.err)
	}
	if !strings.HasPrefix(msg.path, "scratch-") {
		t.Fatalf("scratch path = %q", msg.path)
	}
}

func TestImportCmdCurl(t *testing.T) {
	mgr := newTestManager(t)
	msg := importCmd(context.Background(), mgr, "curl", "curl https://example.com/api", "")().(importMsg)
	if msg.err != nil {
		t.Fatalf("importCmd: %v", msg.err)
	}
	if msg.parsed == nil || len(msg.parsed.Requests) == 0 {
		t.Fatalf("import produced no requests: %+v", msg.parsed)
	}
}

func TestImportCmdUnsupported(t *testing.T) {
	msg := importCmd(context.Background(), newTestManager(t), "bogus", "x", "")().(importMsg)
	if msg.err == nil {
		t.Fatal("unsupported import format should error")
	}
}

func TestStopMockCmd(t *testing.T) {
	msg := stopMockCmd(context.Background(), newTestManager(t))().(simpleMsg)
	if msg.kind != "mock stopped" {
		t.Fatalf("kind = %q", msg.kind)
	}
}

func TestDeleteCookieCmdOpensConfirm(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenCookies)
	m.formInputs[0].SetValue("example.com")
	m.formInputs[2].SetValue("session")
	if cmd := m.deleteCookieCmd(); cmd != nil {
		t.Fatal("deleteCookieCmd should defer to a confirm dialog")
	}
	if m.confirm == nil {
		t.Fatal("deleteCookieCmd should open a confirm dialog")
	}
}

func TestRefreshScreenCmd(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenHistory)
	if m.refreshScreenCmd() == nil {
		t.Fatal("refreshScreenCmd on history should return a load command")
	}
	m.setScreen(screenStress)
	if m.refreshScreenCmd() != nil {
		t.Fatal("refreshScreenCmd on stress should refresh in place (nil cmd)")
	}
}

func TestHandleEventProgress(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.handleEvent(clientmgr.Event{Type: "request_progress", Payload: clientmgr.RequestProgress{Total: 3, Index: 1, RequestName: "x"}})
	if m.progress.Total != 3 || m.progress.Index != 1 {
		t.Fatalf("progress not updated: %+v", m.progress)
	}
	m.handleEvent(clientmgr.Event{Type: "file_changed"}) // must not panic
}

func TestFileChangedEventReloadsSelectedFile(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)
	if _, err := mgr.CreateFile(ctx, "live.http", "### One\nGET https://a.example\n"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	m := newModel(ctx, mgr, Options{})

	// Seed the model with the file list + open the file, mirroring the Update
	// path (filesLoadedMsg -> fileLoadedMsg).
	fm := loadFilesCmd(ctx, mgr)().(filesLoadedMsg)
	if fm.err != nil {
		t.Fatalf("loadFiles: %v", fm.err)
	}
	m.workspace, m.files = fm.workspace, fm.files
	m.refreshFileList()
	if m.selected == "" && len(m.files) > 0 {
		m.selected = m.files[0].RelativePath
	}
	if m.selected != "live.http" {
		t.Fatalf("selected = %q, want live.http", m.selected)
	}
	lm := loadFileCmd(ctx, mgr, m.selected)().(fileLoadedMsg)
	if lm.err != nil {
		t.Fatalf("loadFile: %v", lm.err)
	}
	m.raw, m.parsed = lm.raw, lm.parsed
	m.source.SetValue(lm.raw)
	m.refreshRequestTables()
	if m.raw != "### One\nGET https://a.example\n" {
		t.Fatalf("initial raw = %q", m.raw)
	}

	// External edit on disk (the manager writes + republishes; the model is
	// not updated by this call, so m.raw stays stale).
	if _, err := mgr.SaveFile(ctx, "live.http", "### Two\nGET https://b.example\n"); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if m.raw == "### Two\nGET https://b.example\n" {
		t.Fatal("model should still hold stale content before the event")
	}

	// Fire the file_changed event for the selected file. handleEvent must
	// return a reload command (loadFilesCmd + loadFileCmd).
	cmds := m.handleEvent(clientmgr.Event{
		Type:    "file_changed",
		Payload: clientmgr.FileEvent{Path: "live.http", Operation: "changed"},
	})
	var reload fileLoadedMsg
	found := false
	for _, c := range cmds {
		if c == nil {
			continue
		}
		if fl, ok := c().(fileLoadedMsg); ok {
			reload = fl
			found = true
		}
	}
	if !found {
		t.Fatalf("handleEvent(file_changed) did not return a loadFileCmd; got %d cmds", len(cmds))
	}
	if reload.err != nil {
		t.Fatalf("reload fileLoadedMsg err: %v", reload.err)
	}

	// Apply the reload (mirrors the fileLoadedMsg handler in Update).
	m.raw, m.parsed = reload.raw, reload.parsed
	m.source.SetValue(reload.raw)
	m.refreshRequestTables()
	if m.raw != "### Two\nGET https://b.example\n" {
		t.Fatalf("after event raw = %q, want updated content", m.raw)
	}
	if m.parsed == nil || len(m.parsed.Requests) != 1 || m.parsed.Requests[0].URL != "https://b.example" {
		t.Fatalf("parsed not refreshed: %+v", m.parsed)
	}
}

// TestFileChangedEventSkipsDirtyFile ensures an external file_changed event
// does not clobber the user's unsaved in-progress edits: no loadFileCmd is
// returned for a dirty selected file.
func TestFileChangedEventSkipsDirtyFile(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)
	if _, err := mgr.CreateFile(ctx, "live.http", "### One\nGET https://a.example\n"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	lm := loadFileCmd(ctx, mgr, "live.http")().(fileLoadedMsg)
	if lm.err != nil {
		t.Fatalf("loadFile: %v", lm.err)
	}
	m.selected, m.raw, m.parsed = lm.path, lm.raw, lm.parsed
	m.dirty = true // user has unsaved edits

	if _, err := mgr.SaveFile(ctx, "live.http", "### Two\nGET https://b.example\n"); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	cmds := m.handleEvent(clientmgr.Event{
		Type:    "file_changed",
		Payload: clientmgr.FileEvent{Path: "live.http", Operation: "changed"},
	})
	for _, c := range cmds {
		if c == nil {
			continue
		}
		if _, ok := c().(fileLoadedMsg); ok {
			t.Fatal("dirty file must not be reloaded by a file_changed event")
		}
	}
	if m.raw != "### One\nGET https://a.example\n" {
		t.Fatalf("unsaved edits clobbered: raw = %q", m.raw)
	}
}

func TestFileSummary(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.selected = "api.http"
	m.parsed = &clientmgr.ParsedFileDTO{Requests: []clientmgr.RequestDTO{
		{Name: "Ping", Method: "GET", URL: "https://example.com/api"},
	}}
	out := m.fileSummary()
	for _, want := range []string{"api.http", "GET", "Ping", "https://example.com/api"} {
		if !strings.Contains(out, want) {
			t.Errorf("fileSummary missing %q in:\n%s", want, out)
		}
	}
}

func TestSelectedFiles(t *testing.T) {
	if got := selectedFiles(""); got != nil {
		t.Fatalf("selectedFiles(\"\") = %v, want nil", got)
	}
	if got := selectedFiles("a.http"); len(got) != 1 || got[0] != "a.http" {
		t.Fatalf("selectedFiles(a.http) = %v", got)
	}
}

func TestInitReturnsCommand(t *testing.T) {
	if m0(t, newTestManager(t)).Init() == nil {
		t.Fatal("Init should return a batch command")
	}
}

func TestMediumMainRenders(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.width, m.height = 100, 30
	m.resize()
	if strings.TrimSpace(m.mediumMain(m.geom())) == "" {
		t.Fatal("mediumMain rendered empty")
	}
}

func TestOverlayWrappers(t *testing.T) {
	base := lipgloss.NewStyle().Width(60).Height(20).Render("BASE")
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render("BOX")
	for name, out := range map[string]string{
		"center": overlayCenter(base, 60, 20, box),
		"bottom": overlayBottomRight(base, 60, 20, box),
	} {
		if !strings.Contains(out, "BOX") || !strings.Contains(out, "BASE") {
			t.Errorf("%s overlay lost content:\n%s", name, out)
		}
		if lipgloss.Height(out) != 20 {
			t.Errorf("%s overlay height = %d, want 20", name, lipgloss.Height(out))
		}
	}
}

func TestListItemFormatting(t *testing.T) {
	if got := (fileItem{path: "a.http", name: "a.http", count: 2}).Description(); !strings.Contains(got, "2 requests") {
		t.Errorf("fileItem desc = %q", got)
	}
	if got := (commandItem{title: "Run", desc: "go"}).FilterValue(); got != "Run go" {
		t.Errorf("commandItem filter = %q", got)
	}
	if got := (envItem{name: "dev", active: true}).Title(); !strings.Contains(got, "dev") {
		t.Errorf("envItem title = %q", got)
	}
	if got := (historyItem{id: 5, file: "a.http"}).Title(); !strings.Contains(got, "#5") {
		t.Errorf("historyItem title = %q", got)
	}
	if got := (searchItem{name: "", method: "GET", line: 9}).Title(); !strings.Contains(got, "line 9") {
		t.Errorf("searchItem title fallback = %q", got)
	}
}

// m0 builds a default model for brevity.
func m0(t *testing.T, mgr *clientmgr.Manager) model {
	t.Helper()
	return newModel(context.Background(), mgr, Options{})
}
