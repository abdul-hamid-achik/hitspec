package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

func okAPI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// openModel creates files in mgr, builds a model of the given size and opens the
// first file the way the UI does.
func openModel(t *testing.T, mgr *clientmgr.Manager, w, h int, files map[string]string, open string) model {
	t.Helper()
	ctx := context.Background()
	for name, content := range files {
		if _, err := mgr.CreateFile(ctx, name, content); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	m := newModel(ctx, mgr, Options{})
	m.color = false
	m.width, m.height = w, h
	m.resize()
	list, err := mgr.ListFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m.files = list
	m.refreshFileList()
	next, _ := m.Update(loadFileCmd(ctx, mgr, open)())
	return next.(model)
}

// doneOf runs a run command and returns its runDoneMsg.
func doneOf(t *testing.T, cmd tea.Cmd) runDoneMsg {
	t.Helper()
	for _, msg := range drainBatch(cmd) {
		if d, ok := msg.(runDoneMsg); ok {
			return d
		}
	}
	t.Fatal("command produced no runDoneMsg")
	return runDoneMsg{}
}

func press(m model, k tea.Key) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyPressMsg(k))
	return next.(model), cmd
}

var (
	keyEnter = tea.Key{Code: tea.KeyEnter}
	keyEsc   = tea.Key{Code: tea.KeyEscape}
	keyCtrlC = tea.Key{Code: 'c', Mod: tea.ModCtrl}
	keyCtrlS = tea.Key{Code: 's', Mod: tea.ModCtrl}
)

func rk(r rune) tea.Key { return tea.Key{Text: string(r), Code: r} }

// ---- item 1: run the buffer ------------------------------------------------

func TestRunRequestRunsTheEditedBuffer(t *testing.T) {
	srv := okAPI(t)
	src := "### Ping\n# @name Ping\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, map[string]string{"api.http": src}, "api.http")

	if d := doneOf(t, m.runRequestCmd()); d.err != nil || d.result.Passed != 1 {
		t.Fatalf("saved file should pass: err=%v result=%+v", d.err, d.result)
	}
	m.endRun()

	// Edit the assertion to a status the server never returns, without saving.
	m.source.SetValue(strings.Replace(src, "expect status 200", "expect status 4041", 1))
	m.dirty = m.bufferModified()
	d := doneOf(t, m.runRequestCmd())
	if d.err != nil || d.result.Failed != 1 || d.result.Passed != 0 {
		t.Fatalf("the edited buffer must run (4041 fails): err=%v result=%+v", d.err, d.result)
	}
	if onDisk, _ := mgr.ReadFile(context.Background(), "api.http"); onDisk != src {
		t.Fatal("running must not save the buffer")
	}
}

func TestRunRequestRunsEditedUntitledRequest(t *testing.T) {
	srv := okAPI(t)
	src := "### x\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	m := openModel(t, newTestManager(t), 120, 40, map[string]string{"u.http": src}, "u.http")
	m.source.SetValue(strings.Replace(src, "expect status 200", "expect status 4041", 1))
	m.dirty = true
	d := doneOf(t, m.runRequestCmd())
	if d.err != nil || d.result.Failed != 1 {
		t.Fatalf("untitled request in the buffer: err=%v result=%+v", d.err, d.result)
	}
}

func TestRunFileRunsTheEditedBuffer(t *testing.T) {
	srv := okAPI(t)
	src := "### a\n# @name a\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	m := openModel(t, newTestManager(t), 120, 40, map[string]string{"f.http": src}, "f.http")
	m.source.SetValue(src + "\n### b\n# @name b\nGET " + srv.URL + "\n\n>>>\nexpect status 500\n<<<\n")
	m.dirty = true
	d := doneOf(t, m.runFileCmd())
	if d.err != nil || len(d.result.Results) != 2 || d.result.Failed != 1 {
		t.Fatalf("whole-file run of the buffer: err=%v result=%+v", d.err, d.result)
	}
}

func TestRunBufferParseErrorIsReported(t *testing.T) {
	src := "### a\nGET http://127.0.0.1:1\n"
	m := openModel(t, newTestManager(t), 120, 40, map[string]string{"p.http": src}, "p.http")
	m.source.SetValue("### a\nNOPE\n")
	m.dirty = true
	d := doneOf(t, m.runRequestCmd())
	if d.err == nil {
		t.Fatal("an unparseable buffer should fail the run with an error")
	}
	next, _ := m.Update(d)
	nm := next.(model)
	if nm.err == "" || !strings.Contains(plain(nm.respView.view()), "NOPE") && !strings.Contains(plain(nm.respView.view()), "parse") {
		t.Fatalf("the parse error should be shown in the response pane: err=%q view=%q", nm.err, plain(nm.respView.view()))
	}
}

// ---- item 2: lost edits ----------------------------------------------------

func twoFiles(srv string) map[string]string {
	return map[string]string{
		"a.http": "### A\n# @name A\nGET " + srv + "\n",
		"b.http": "### B\n# @name B\nGET " + srv + "\n",
	}
}

func TestOpeningAnotherFileAsksWhenDirty(t *testing.T) {
	srv := okAPI(t)
	m := openModel(t, newTestManager(t), 120, 40, twoFiles(srv.URL), "a.http")
	m.source.SetValue(m.raw + "# edit\n")
	m.dirty = true
	m.filesList.Select(1) // b.http

	m, cmd := press(m, keyEnter)
	if m.confirm == nil || cmd != nil {
		t.Fatalf("enter on another file with unsaved edits must ask first (confirm=%v cmd=%v)", m.confirm, cmd != nil)
	}
	if m.selected != "a.http" || !m.dirty {
		t.Fatal("nothing may change before the user answers")
	}

	// n: cancelled, still on a.http with the edits.
	m, _ = press(m, rk('n'))
	if m.confirm != nil || m.selected != "a.http" || !m.dirty || m.source.Value() == m.raw {
		t.Fatal("declining must keep the buffer")
	}

	// y: the file opens.
	m, _ = press(m, keyEnter)
	m, cmd = press(m, rk('y'))
	if cmd == nil {
		t.Fatal("confirming should return the guarded operation")
	}
	next, cmd := m.Update(cmd())
	loaded := cmd().(fileLoadedMsg)
	if loaded.path != "b.http" || loaded.err != nil {
		t.Fatalf("confirmed open loaded %+v", loaded)
	}
	m = next.(model)
	next, _ = m.Update(loaded)
	m = next.(model)
	if m.selected != "b.http" || m.dirty {
		t.Fatalf("after confirming, b.http is open and clean: selected=%s dirty=%v", m.selected, m.dirty)
	}
}

func TestOpeningAFileWhenCleanDoesNotAsk(t *testing.T) {
	srv := okAPI(t)
	m := openModel(t, newTestManager(t), 120, 40, twoFiles(srv.URL), "a.http")
	m.filesList.Select(1)
	m, cmd := press(m, keyEnter)
	if m.confirm != nil || cmd == nil {
		t.Fatal("a clean buffer opens the file straight away")
	}
}

func TestEveryBufferReplacingFlowAsks(t *testing.T) {
	srv := okAPI(t)
	dirtyModel := func() model {
		m := openModel(t, newTestManager(t), 120, 40, twoFiles(srv.URL), "a.http")
		m.source.SetValue(m.raw + "# edit\n")
		m.dirty = true
		return m
	}
	t.Run("new file", func(t *testing.T) {
		m := dirtyModel()
		m, _ = press(m, rk('n'))
		if m.confirm == nil {
			t.Fatal("n with unsaved edits must ask")
		}
	})
	t.Run("search", func(t *testing.T) {
		m := dirtyModel()
		m.searchOpen = true
		_ = m.searchList.SetItems([]list.Item{searchItem{file: "b.http", name: "B", method: "GET"}})
		m, cmd := press(m, keyEnter)
		if m.confirm == nil || cmd != nil {
			t.Fatal("selecting a search result with unsaved edits must ask")
		}
	})
	t.Run("rename and duplicate", func(t *testing.T) {
		for _, id := range []string{"rename-file", "duplicate-file"} {
			m := dirtyModel()
			m.executeCommand(id)
			if m.prompt == nil {
				t.Fatalf("%s should open its prompt", id)
			}
			next, cmd := m.Update(m.prompt.onSubmit("c.http")())
			_ = cmd
			if next.(model).confirm == nil {
				t.Fatalf("%s submit with unsaved edits must ask", id)
			}
		}
	})
	t.Run("import", func(t *testing.T) {
		m := dirtyModel()
		m.setScreen(screenImport)
		m.formInputs[1].SetValue("curl https://example.com")
		if cmd := m.submitFormCmd(); cmd != nil || m.confirm == nil {
			t.Fatal("import with unsaved edits must ask")
		}
	})
	t.Run("scaffold", func(t *testing.T) {
		m := dirtyModel()
		if cmd := m.executeCommand("scaffold"); cmd != nil || m.confirm == nil {
			t.Fatal("generate sample with unsaved edits must ask")
		}
	})
	t.Run("ctrl+c", func(t *testing.T) {
		m := dirtyModel()
		m, cmd := press(m, keyCtrlC)
		if cmd != nil || m.confirm == nil || !m.confirm.forQuit {
			t.Fatal("ctrl+c with unsaved edits must ask")
		}
	})
	t.Run("import clears dirty", func(t *testing.T) {
		m := dirtyModel()
		next, _ := m.Update(importMsg{path: "imported.http", raw: "### i\nGET http://x\n"})
		if nm := next.(model); nm.dirty {
			t.Fatal("an imported buffer replaces the edits, so it must not stay dirty")
		}
	})
}

func TestRevertingAnEditIsNotDirty(t *testing.T) {
	srv := okAPI(t)
	m := openModel(t, newTestManager(t), 120, 40, twoFiles(srv.URL), "a.http")
	m, _ = press(m, rk('e'))
	m, _ = press(m, rk('x'))
	if !m.dirty {
		t.Fatal("typing makes the buffer dirty")
	}
	m, _ = press(m, tea.Key{Code: tea.KeyBackspace})
	if m.dirty {
		t.Fatal("deleting the typed character restores the file's text, so nothing is unsaved")
	}
}

func TestSaveAsksBeforeOverwritingExternalChange(t *testing.T) {
	srv := okAPI(t)
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, twoFiles(srv.URL), "a.http")
	ctx := context.Background()

	mine := m.raw + "# mine\n"
	m.source.SetValue(mine)
	m.dirty = true

	// Someone else changes the file after it was loaded.
	if _, err := mgr.SaveFile(ctx, "a.http", m.raw+"# theirs\n"); err != nil {
		t.Fatal(err)
	}

	msg := m.saveCmd()()
	conflict, ok := msg.(saveConflictMsg)
	if !ok {
		t.Fatalf("save over an external change must report a conflict, got %T", msg)
	}
	if disk, _ := mgr.ReadFile(ctx, "a.http"); !strings.Contains(disk, "# theirs") {
		t.Fatal("their change must still be on disk")
	}
	next, _ := m.Update(conflict)
	nm := next.(model)
	if nm.confirm == nil || !strings.Contains(nm.confirm.title, "changed on disk") {
		t.Fatalf("expected the overwrite dialog, got %+v", nm.confirm)
	}
	// Decline: still theirs.
	nm, _ = press(nm, rk('n'))
	if disk, _ := mgr.ReadFile(ctx, "a.http"); !strings.Contains(disk, "# theirs") {
		t.Fatal("declining must not overwrite")
	}
	// Accept: mine wins.
	next, _ = m.Update(conflict)
	nm = next.(model)
	nm, cmd := press(nm, rk('y'))
	if _, ok := cmd().(fileLoadedMsg); !ok {
		t.Fatal("confirming should save")
	}
	if disk, _ := mgr.ReadFile(ctx, "a.http"); disk != mine {
		t.Fatalf("overwrite should write my buffer, disk=%q", disk)
	}
}

func TestSaveWithoutExternalChangeIsImmediate(t *testing.T) {
	srv := okAPI(t)
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, twoFiles(srv.URL), "a.http")
	m.source.SetValue(m.raw + "# mine\n")
	if _, ok := m.saveCmd()().(fileLoadedMsg); !ok {
		t.Fatal("an unchanged file saves without asking")
	}
}

// ---- item 3: small terminals -----------------------------------------------

func requestsOf(srv string) map[string]string {
	return map[string]string{"api.http": "### One\n# @name One\nGET " + srv + "/1\n\n### Two\n# @name Two\nGET " + srv + "/2\n\n### Three\n# @name Three\nGET " + srv + "/3\n"}
}

func TestSmallTerminalShowsRequestRows(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 24}, {60, 24}, {120, 24}} {
		m := openModel(t, newTestManager(t), sz[0], sz[1], requestsOf("http://x"), "api.http")
		m.focus = focusRequests
		m.syncFocus()
		view := plain(m.View().Content)
		for _, name := range []string{"One", "Two", "Three"} {
			if !strings.Contains(view, name) {
				t.Fatalf("%dx%d: request %q is not visible:\n%s", sz[0], sz[1], name, view)
			}
		}
	}
}

func TestRunBringsTheResponseIntoViewBelow120Columns(t *testing.T) {
	for _, w := range []int{60, 80, 100, 119} {
		m := openModel(t, newTestManager(t), w, 30, requestsOf("http://x"), "api.http")
		next, _ := m.Update(runDoneMsg{result: sampleResult()})
		nm := next.(model)
		if nm.focus != focusResponse {
			t.Fatalf("width %d: after a run the response pane should be focused, got %v", w, nm.focus)
		}
		if !strings.Contains(plain(nm.View().Content), "hello") {
			t.Fatalf("width %d: the response body is not on screen", w)
		}
	}
	m := openModel(t, newTestManager(t), 140, 40, requestsOf("http://x"), "api.http")
	m.focus = focusRequests
	next, _ := m.Update(runDoneMsg{result: sampleResult()})
	if next.(model).focus != focusRequests {
		t.Fatal("the wide layout shows the response already; focus must not move")
	}
}

func TestRunErrorIsShownInFullAndWrapped(t *testing.T) {
	long := "failed to parse api.http: " + strings.Repeat("unexpected token somewhere in the file ", 6)
	m := openModel(t, newTestManager(t), 100, 30, requestsOf("http://x"), "api.http")
	next, _ := m.Update(runDoneMsg{err: fmt.Errorf("%s", long)})
	nm := next.(model)
	view := plain(nm.View().Content)
	if nm.focus != focusResponse {
		t.Fatal("the error should be brought into view")
	}
	// The tail of the message must be readable somewhere, wrapped across lines.
	if !strings.Contains(view, "somewhere in") || strings.Count(view, "unexpected token") < 4 {
		t.Fatalf("the full error should be visible in the response pane:\n%s", view)
	}
}

// ---- item 4: big JSON ------------------------------------------------------

func TestPrepareBodyPrettyPrintsBeforeTruncating(t *testing.T) {
	items := make([]map[string]any, 0, 3000)
	for i := 0; i < 3000; i++ {
		items = append(items, map[string]any{"id": i, "name": fmt.Sprintf("item-%d", i)})
	}
	raw, _ := json.Marshal(items)
	if len(raw) < 24000 {
		t.Fatal("test body should be larger than the old 24 KB cut")
	}
	text, lexer, cut := prepareBody(string(raw), "application/json")
	if lexer != "json" || cut == nil {
		t.Fatalf("a long JSON array should be cut (lexer=%q cut=%v)", lexer, cut)
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxBodyLines || len(lines) < 100 {
		t.Fatalf("expected a pretty-printed, line-limited body, got %d lines", len(lines))
	}
	if !strings.HasPrefix(text, "[\n  {\n    \"id\": 0,") {
		t.Fatalf("body should be pretty-printed, starts %q", text[:40])
	}
	for _, l := range lines {
		if len(l) > 200 {
			t.Fatalf("no line should be a giant minified run: %d bytes", len(l))
		}
	}
	msg := cut.String()
	for _, want := range []string{"truncated", fmt.Sprint(cut.totalLines), "lines"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("the cut notice should say how much was left out (%q): %s", want, msg)
		}
	}
	if cut.shownLines != len(lines) || cut.totalLines <= cut.shownLines {
		t.Fatalf("bad accounting: %+v vs %d shown lines", cut, len(lines))
	}
}

func TestPrepareBodyKeepsABigButShortJSONWhole(t *testing.T) {
	obj := map[string]string{}
	for i := 0; i < 80; i++ {
		obj[fmt.Sprintf("k%02d", i)] = strings.Repeat("v", 400) // ~33 KB minified, 82 lines pretty
	}
	raw, _ := json.Marshal(obj)
	if len(raw) < 24000 || len(raw) > maxBodyBytes {
		t.Fatalf("setup: %d bytes", len(raw))
	}
	text, _, cut := prepareBody(string(raw), "application/json")
	if cut != nil {
		t.Fatalf("a 33 KB body of 82 lines is not truncated: %+v", cut)
	}
	if !json.Valid([]byte(text)) {
		t.Fatal("what is shown must still be valid JSON")
	}
}

func TestPrepareBodyCutsHugeSingleLine(t *testing.T) {
	text, _, cut := prepareBody(strings.Repeat("é", maxBodyBytes), "text/plain") // 2 bytes per rune, one line
	if cut == nil || len(text) > maxBodyBytes {
		t.Fatalf("a huge single line must be cut (len=%d cut=%v)", len(text), cut)
	}
	if !strings.HasSuffix(text, "é") {
		t.Fatal("the cut must land on a rune boundary")
	}
}

func TestResponseViewerSoftWrapsLongLines(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setSize(24, 12)
	long := strings.Repeat("abcdefghij", 12) // 120 chars, no spaces
	rv.setResult(&clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{{
		Name: "x", Passed: true, Response: &clientmgr.HTTPResponseDTO{StatusCode: 200, Status: "200 OK", Body: long},
	}}})
	view := plain(rv.view())
	if got := strings.Count(view, "j"); got != 12 {
		t.Fatalf("a long line must wrap, not be cut: %d of 12 chunks visible\n%s", got, view)
	}
}

// ---- item 5: screens 2-9 ---------------------------------------------------

func TestFormsStartEmptyWithPlaceholders(t *testing.T) {
	for _, s := range []screen{screenStress, screenMock, screenContract, screenRecord, screenImport, screenCookies} {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.setScreen(s)
		if len(m.formInputs) == 0 {
			t.Fatalf("%s has no form", screenNames[s])
		}
		for _, in := range m.formInputs {
			if in.Value() != "" {
				t.Fatalf("%s: field %q is pre-filled with %q", screenNames[s], in.Prompt, in.Value())
			}
		}
	}
}

func TestEnterNeverSubmitsAForm(t *testing.T) {
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, requestsOf("http://x"), "api.http")
	m.setScreen(screenStress)

	// Not editing: enter opens the fields, it does not start a load test.
	m, _ = press(m, keyEnter)
	if !m.formActive {
		t.Fatal("enter on a form screen should open its fields")
	}
	// Editing: enter moves on to the next field.
	m, cmd := press(m, keyEnter)
	if cmd != nil || m.formFocus != 1 {
		t.Fatalf("enter inside the form moves to the next field (focus=%d cmd=%v)", m.formFocus, cmd != nil)
	}
	if mgr.StressStatus(context.Background()).Running {
		t.Fatal("enter started a stress run")
	}
	// ctrl+s is the explicit submit while editing.
	m, cmd = press(m, keyCtrlS)
	if cmd == nil || m.formActive {
		t.Fatal("ctrl+s submits the form")
	}
}

func TestSubmitValidatesRequiredFields(t *testing.T) {
	cases := map[screen]string{
		screenContract: "provider URL is required",
		screenRecord:   "target URL is required",
		screenImport:   "input is required",
		screenCookies:  "domain is required",
	}
	for s, want := range cases {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.setScreen(s)
		m.submitFormCmd()
		if m.err != want {
			t.Fatalf("%s: err = %q, want %q", screenNames[s], m.err, want)
		}
	}
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenMock)
	m.formInputs[0].SetValue("abc")
	m.submitFormCmd()
	if !strings.Contains(m.err, "port must be") {
		t.Fatalf("a non-numeric port should be rejected, err=%q", m.err)
	}
}

func TestEscGoesBackFromSecondaryScreens(t *testing.T) {
	for _, s := range []screen{screenStress, screenMock, screenContract, screenRecord, screenHistory, screenImport, screenCookies, screenSettings} {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.setScreen(s)
		m, _ = press(m, keyEsc)
		if m.screen != screenWorkspace {
			t.Fatalf("esc on %s should go back to the workspace, screen=%v", screenNames[s], m.screen)
		}
	}
	// With the fields open, the first esc only leaves them.
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenMock)
	m, _ = press(m, rk('e'))
	m, _ = press(m, keyEsc)
	if m.screen != screenMock || m.formActive {
		t.Fatalf("first esc leaves the fields only: screen=%v active=%v", m.screen, m.formActive)
	}
	m, _ = press(m, keyEsc)
	if m.screen != screenWorkspace {
		t.Fatal("second esc goes back")
	}
}

// ---- item 6: panels --------------------------------------------------------

func TestHeaderAndAssertionPanelsFollowTheSelectedRequest(t *testing.T) {
	src := "### One\n# @name One\nGET http://x/1\nX-One: 1\n\n>>>\nexpect status 200\n<<<\n\n" +
		"### Two\n# @name Two\nGET http://x/2\nX-Two: 2\n\n>>>\nexpect status 201\nexpect body.id exists\n<<<\n"
	m := openModel(t, newTestManager(t), 140, 50, map[string]string{"api.http": src}, "api.http")
	m.focus = focusRequests
	m.syncFocus()

	first := m.headers.Rows()
	if len(first) != 1 || first[0][0] != "X-One" || len(m.assertions.Rows()) != 1 {
		t.Fatalf("first request: headers=%v assertions=%v", first, m.assertions.Rows())
	}
	m, _ = press(m, tea.Key{Code: tea.KeyDown})
	hs := m.headers.Rows()
	if len(hs) != 1 || hs[0][0] != "X-Two" || len(m.assertions.Rows()) != 2 {
		t.Fatalf("after moving to the second request: headers=%v assertions=%v", hs, m.assertions.Rows())
	}

	// After a run, each request shows its own response.
	m.lastResult = &clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{
		{Name: "One", Passed: true, Response: &clientmgr.HTTPResponseDTO{StatusCode: 200, Headers: map[string]string{"A-One": "1"}}},
		{Name: "Two", Passed: false, Response: &clientmgr.HTTPResponseDTO{StatusCode: 500, Headers: map[string]string{"B-Two": "2"}},
			Assertions: []clientmgr.AssertionResultDTO{{Subject: "status", Operator: "==", Expected: 201}}},
	}}
	m.refreshResultViews()
	if hs := m.headers.Rows(); len(hs) != 1 || hs[0][0] != "B-Two" || !m.metaFromRun {
		t.Fatalf("selected request two should show its response headers: %v", hs)
	}
	m, _ = press(m, tea.Key{Code: tea.KeyUp})
	if hs := m.headers.Rows(); len(hs) != 1 || hs[0][0] != "A-One" {
		t.Fatalf("selected request one should show its own response headers: %v", hs)
	}
}

func TestRunFileResultsAreAllReachable(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setSize(60, 20)
	mk := func(name string, status int, body string, pass bool) clientmgr.RequestResultDTO {
		return clientmgr.RequestResultDTO{Name: name, Passed: pass, Response: &clientmgr.HTTPResponseDTO{StatusCode: status, Status: "x", Body: body}}
	}
	rv.setResult(&clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{
		mk("first", 200, "BODY-FIRST", true), mk("second", 500, "BODY-SECOND", false), mk("third", 200, "BODY-THIRD", true),
	}})
	if v := plain(rv.view()); !strings.Contains(v, "BODY-SECOND") || !strings.Contains(v, "request 2/3") {
		t.Fatalf("a run opens on its first failure:\n%s", v)
	}
	rv.selectRelative(1)
	if v := plain(rv.view()); !strings.Contains(v, "BODY-THIRD") || !strings.Contains(v, "request 3/3") {
		t.Fatalf("next request:\n%s", v)
	}
	rv.selectRelative(1) // clamped
	rv.selectRelative(-2)
	if v := plain(rv.view()); !strings.Contains(v, "BODY-FIRST") {
		t.Fatalf("previous requests:\n%s", v)
	}

	// The keys work from the response pane.
	m := openModel(t, newTestManager(t), 140, 40, requestsOf("http://x"), "api.http")
	next, _ := m.Update(runDoneMsg{result: &clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{
		mk("a", 200, "AAA", true), mk("b", 200, "BBB", true)}}})
	m = next.(model)
	m.focus = focusResponse
	m, _ = press(m, rk('.'))
	if !strings.Contains(plain(m.respView.view()), "BBB") {
		t.Fatal(". should show the next request's response")
	}
	m, _ = press(m, rk(','))
	if !strings.Contains(plain(m.respView.view()), "AAA") {
		t.Fatal(", should show the previous request's response")
	}
}

// ---- item 7: cancellation --------------------------------------------------

func slowAPI(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(20 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestXCancelsARunAndASecondRunIsIgnored(t *testing.T) {
	srv, hits := slowAPI(t)
	m := openModel(t, newTestManager(t), 140, 40, map[string]string{"s.http": "### S\n# @name S\nGET " + srv.URL + "\n"}, "s.http")

	cmd := m.runRequestCmd()
	if !m.running {
		t.Fatal("a started run is marked running")
	}
	done := make(chan runDoneMsg, 1)
	go func() { done <- doneOf(t, cmd) }()
	waitFor(t, "the request to reach the server", func() bool { return hits.Load() == 1 })

	// A second r is ignored, not started concurrently.
	m, second := press(m, rk('r'))
	if second == nil {
		t.Fatal("ignoring the second run should still tell the user")
	}
	time.Sleep(100 * time.Millisecond)
	if hits.Load() != 1 {
		t.Fatalf("a second run must not start: %d requests reached the server", hits.Load())
	}
	// R as well.
	if _, c := press(m, rk('R')); c == nil || hits.Load() != 1 {
		t.Fatal("R during a run is ignored too")
	}

	m, _ = press(m, rk('x'))
	if !m.cancelling {
		t.Fatal("x should cancel the run")
	}
	var d runDoneMsg
	select {
	case d = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop after cancel")
	}
	next, _ := m.Update(d)
	m = next.(model)
	if m.running || m.cancelling || m.loading {
		t.Fatalf("a finished run clears the in-flight state: %+v", m.running)
	}
	if m.progress.Total != 0 {
		t.Fatal("stale progress must be cleared")
	}
	if m.err != "" && !strings.Contains(m.status, "cancel") {
		t.Fatalf("a cancel is not an error: err=%q status=%q", m.err, m.status)
	}
	// A new run can start again.
	if _, _, ok := m.beginRun(); !ok {
		t.Fatal("after the run ended another can start")
	}
}

func TestCtrlCCancelsARunThenQuits(t *testing.T) {
	srv, hits := slowAPI(t)
	m := openModel(t, newTestManager(t), 140, 40, map[string]string{"s.http": "### S\n# @name S\nGET " + srv.URL + "\n"}, "s.http")
	cmd := m.runRequestCmd()
	done := make(chan struct{})
	go func() { defer close(done); drainBatch(cmd) }()
	waitFor(t, "the request to reach the server", func() bool { return hits.Load() == 1 })

	m, c := press(m, keyCtrlC)
	if !m.cancelling {
		t.Fatal("the first ctrl+c cancels the run")
	}
	if c != nil {
		if _, quit := c().(tea.QuitMsg); quit {
			t.Fatal("the first ctrl+c must not quit while a run is in flight")
		}
	}
	<-done
	// The run was asked to stop; if it does not finish, the next ctrl+c quits.
	_, c = press(m, keyCtrlC)
	if c == nil {
		t.Fatal("second ctrl+c should quit")
	}
	if _, quit := c().(tea.QuitMsg); !quit {
		t.Fatal("second ctrl+c should quit")
	}
}

func TestProgressDoesNotOutliveTheRun(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	// A late event after the run ended is ignored.
	m.handleEvent(clientmgr.Event{Type: "request_progress", Payload: clientmgr.RequestProgress{Total: 3, Index: 3, RequestName: "late"}})
	if m.progress.Total != 0 {
		t.Fatal("progress with no run in flight must be ignored")
	}
	m.running = true
	m.handleEvent(clientmgr.Event{Type: "request_progress", Payload: clientmgr.RequestProgress{Total: 3, Index: 1, RequestName: "x"}})
	if !strings.Contains(m.statusbar(), "x 1/3") {
		t.Fatalf("progress is shown during a run: %q", plain(m.statusbar()))
	}
	// An error ends the run too and must clear the bar.
	next, _ := m.Update(runDoneMsg{err: fmt.Errorf("boom")})
	nm := next.(model)
	if nm.progress.Total != 0 || strings.Contains(plain(nm.statusbar()), "1/3") {
		t.Fatalf("stale progress after a failed run: %q", plain(nm.statusbar()))
	}
}

// ---- item 8: copy as -------------------------------------------------------

func TestCopyAsWorksForUnnamedRequestsAndResolvesVariables(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hitspec.yaml"), []byte("defaultEnvironment: dev\nenvironments:\n  dev:\n    baseUrl: http://dev.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := clientmgr.New(clientmgr.WithWorkDir(dir), clientmgr.WithWatch(false), clientmgr.WithHistoryDBPath(filepath.Join(dir, "h.db")))
	mgr.Start(context.Background())
	t.Cleanup(func() { _ = mgr.Close() })

	src := "### first\n# @name first\nGET {{baseUrl}}/first\n\n###\nGET {{baseUrl}}/untitled\n"
	m := openModel(t, mgr, 140, 40, map[string]string{"u.http": src}, "u.http")
	m.focus = focusRequests
	m.requests.SetCursor(1) // the untitled one

	cm, ok := m.exportSelected("curl")().(copyMsg)
	if !ok || cm.err != nil {
		t.Fatalf("copy as curl of an untitled request: %+v", cm)
	}
	if !strings.Contains(cm.content, "http://dev.example/untitled") || strings.Contains(cm.content, "{{") {
		t.Fatalf("the snippet must be the selected request with variables resolved: %s", cm.content)
	}
	if strings.Contains(cm.content, "/first") {
		t.Fatalf("only the selected request is exported: %s", cm.content)
	}

	// The edited (unsaved) buffer is what gets exported.
	m.source.SetValue(strings.Replace(src, "/untitled", "/edited", 1))
	cm = m.exportSelected("curl")().(copyMsg)
	if !strings.Contains(cm.content, "/edited") {
		t.Fatalf("copy-as should follow the buffer: %s", cm.content)
	}
}

// ---- item 9 / 10 -----------------------------------------------------------

func TestMouseCaptureIsOffUnlessRequested(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("mouse capture must be off by default so text can be selected")
	}
	m = newModel(context.Background(), newTestManager(t), Options{Mouse: true})
	if m.View().MouseMode == tea.MouseModeNone {
		t.Fatal("Options.Mouse turns capture on")
	}
}

func TestCookiesScreenSaysTheStoreIsGlobal(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	got := plain(m.cookiesContent())
	for _, want := range []string{"tui-cookies.json", "every workspace", "not sent"} {
		if !strings.Contains(got, want) {
			t.Fatalf("cookies screen should say %q:\n%s", want, got)
		}
	}
}

// ---- review follow-ups -----------------------------------------------------

// A buffer that does not parse is still written (SaveFile parses afterwards), so
// the editor must treat it as saved: the next ctrl+s is not an outside change.
func TestSavingUnparseableBufferThenFixingItIsNotAConflict(t *testing.T) {
	srv := okAPI(t)
	mgr := newTestManager(t)
	m := openModel(t, mgr, 120, 40, twoFiles(srv.URL), "a.http")

	m.source.SetValue("### A\nNOPE\n")
	m.dirty = m.bufferModified()
	msg := m.saveCmd()()
	un, ok := msg.(fileSavedUnparsedMsg)
	if !ok {
		t.Fatalf("saving an unparseable buffer should report it was saved but does not parse, got %T", msg)
	}
	next, _ := m.Update(un)
	m = next.(model)
	if m.raw != "### A\nNOPE\n" || m.dirty {
		t.Fatalf("the buffer now matches the file: raw=%q dirty=%v", m.raw, m.dirty)
	}
	if !strings.Contains(m.err, "does not parse") {
		t.Fatalf("the parse error should be shown, err=%q", m.err)
	}

	// Fix the typo and save again: no "changed on disk" dialog.
	m.source.SetValue("### A\n# @name A\nGET " + srv.URL + "\n")
	m.dirty = m.bufferModified()
	if _, ok := m.saveCmd()().(fileLoadedMsg); !ok {
		t.Fatalf("saving the fixed buffer must not report a conflict")
	}
}

func TestCopyResponseBodyCopiesTheShownResponse(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	mk := func(name, body string, pass bool) clientmgr.RequestResultDTO {
		return clientmgr.RequestResultDTO{Name: name, Passed: pass, Response: &clientmgr.HTTPResponseDTO{StatusCode: 200, Body: body}}
	}
	m.lastResult = &clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{mk("a", "BODY-A", true), mk("b", "BODY-B", true), mk("c", "BODY-C", false)}}
	m.refreshResultViews()
	if got := m.lastResponseBody(); got != "BODY-C" { // opens on the first failure
		t.Fatalf("copy should take the shown (failed) response, got %q", got)
	}
	m.respView.selectRelative(-2)
	if got := m.lastResponseBody(); got != "BODY-A" {
		t.Fatalf("copy should follow the selection, got %q", got)
	}
}

func TestErrorToastsWrapInsteadOfTruncating(t *testing.T) {
	long := "the quick brown fox jumps over the lazy dog and keeps running through the whole sentence until the end"
	c, _ := newToastCenter().push(toastError, long)
	got := c.items[0].text
	if !strings.Contains(got, "\n") || !strings.Contains(got, "the end") {
		t.Fatalf("an error toast should wrap and keep its tail: %q", got)
	}
	c, _ = newToastCenter().push(toastError, strings.Repeat("word ", 200))
	if n := strings.Count(c.items[0].text, "\n") + 1; n > toastMaxLines {
		t.Fatalf("a wrapped toast is capped at %d lines, got %d", toastMaxLines, n)
	}
	c, _ = newToastCenter().push(toastSuccess, long)
	if strings.Contains(c.items[0].text, "\n") {
		t.Fatal("non-error toasts stay one line")
	}
}
