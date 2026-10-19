package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

func TestSearchOpensOnCtrlF(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'f', Mod: tea.ModCtrl}))
	if !next.(model).searchOpen {
		t.Fatal("ctrl+f should open the search overlay")
	}
}

func TestSearchResultMsgPopulatesAndIgnoresStale(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.searchOpen = true
	m.searchInput.SetValue("users")

	next, _ := m.Update(searchResultMsg{query: "users", results: []clientmgr.SearchResultDTO{
		{File: "a.http", RequestName: "List", Method: "GET", URL: "https://x/users"},
	}})
	nm := next.(model)
	if len(nm.searchList.Items()) != 1 {
		t.Fatalf("matching query should populate the list, got %d", len(nm.searchList.Items()))
	}

	// A response for a query the user has moved on from must be ignored.
	next2, _ := nm.Update(searchResultMsg{query: "stale", results: []clientmgr.SearchResultDTO{
		{File: "b.http"}, {File: "c.http"},
	}})
	if len(next2.(model).searchList.Items()) != 1 {
		t.Fatal("stale search results should be ignored")
	}
}

func TestSearchTypingReachesInput(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.searchOpen = true
	_ = m.searchInput.Focus()
	next, _ := m.Update(keyPress('u'))
	if got := next.(model).searchInput.Value(); got != "u" {
		t.Fatalf("typing should reach the search input, got %q", got)
	}
}

func TestSearchEnterOpensSelectedFile(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "found.http", sampleHTTP); err != nil {
		t.Fatalf("create: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.searchOpen = true
	m.searchResults = []clientmgr.SearchResultDTO{
		{File: "found.http", RequestName: "Ping", Method: "GET", URL: "https://example.com/api/widgets"},
	}
	m.refreshSearchList()

	cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if m.searchOpen {
		t.Fatal("enter should close the search overlay")
	}
	if cmd == nil {
		t.Fatal("enter should return a load command")
	}
	flm, ok := cmd().(fileLoadedMsg)
	if !ok {
		t.Fatalf("want fileLoadedMsg, got %T", cmd())
	}
	if flm.path != "found.http" || flm.err != nil {
		t.Fatalf("unexpected fileLoadedMsg: %+v", flm)
	}
}

func TestSearchCmdEndToEnd(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "u.http", "### List users\nGET https://api.example.com/users\n"); err != nil {
		t.Fatalf("create: %v", err)
	}
	msg := searchRequestsCmd(ctx, mgr, "users")().(searchResultMsg)
	if msg.err != nil {
		t.Fatalf("search cmd: %v", msg.err)
	}
	if msg.query != "users" || len(msg.results) != 1 {
		t.Fatalf("unexpected search result: %+v", msg)
	}
}

const searchTwoFiles = "### First\n# @name first\nGET https://example.com/one\n\n### Second\n# @name second\nGET https://example.com/two\n\n###\nGET https://example.com/three\n"

// searchSelect drives enter on the first search result and feeds the load
// command's message back through Update, as the Bubble Tea runtime would.
func searchSelect(t *testing.T, m model, r clientmgr.SearchResultDTO) model {
	t.Helper()
	m.searchOpen = true
	m.searchResults = []clientmgr.SearchResultDTO{r}
	m.refreshSearchList()
	cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		next, _ := m.Update(cmd())
		m = next.(model)
	}
	return m
}

func TestSearchSelectsRequestInAnotherFile(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.CreateFile(ctx, "b.http", searchTwoFiles); err != nil {
		t.Fatal(err)
	}
	hits, err := mgr.SearchRequests(ctx, "two")
	if err != nil || len(hits) != 1 {
		t.Fatalf("search: %v %v", hits, err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "a.http"
	m = searchSelect(t, m, hits[0])
	if m.selected != "b.http" {
		t.Fatalf("selected = %q, want b.http", m.selected)
	}
	if got := m.requests.Cursor(); got != 1 {
		t.Fatalf("cursor = %d, want 1 (the second request)", got)
	}
	if m.focus != focusRequests {
		t.Fatalf("focus = %v, want the request list", m.focus)
	}
}

func TestSearchSelectsUntitledRequestByLine(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "b.http", searchTwoFiles); err != nil {
		t.Fatal(err)
	}
	hits, _ := mgr.SearchRequests(ctx, "three")
	if len(hits) != 1 || hits[0].RequestName != "" {
		t.Fatalf("want one untitled hit, got %+v", hits)
	}
	m := newModel(ctx, mgr, Options{})
	m = searchSelect(t, m, hits[0])
	if got := m.requests.Cursor(); got != 2 {
		t.Fatalf("cursor = %d, want 2", got)
	}
}

// A hit in the file that is already open moves the cursor without reloading it
// and without asking to discard the edits in the buffer.
func TestSearchSelectsRequestInOpenFileWithoutDiscardPrompt(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "b.http", searchTwoFiles); err != nil {
		t.Fatal(err)
	}
	hits, _ := mgr.SearchRequests(ctx, "two")
	m := newModel(ctx, mgr, Options{})
	loaded, ok := loadFileCmd(ctx, mgr, "b.http")().(fileLoadedMsg)
	if !ok {
		t.Fatal("load b.http")
	}
	m.openFile(loaded.path, loaded.raw, loaded.parsed)
	m.dirty = true
	m = searchSelect(t, m, hits[0])
	if m.confirm != nil {
		t.Fatal("a hit in the open file must not prompt to discard")
	}
	if got := m.requests.Cursor(); got != 1 {
		t.Fatalf("cursor = %d, want 1", got)
	}
}

// Declining the discard prompt leaves no stale jump behind: a later plain open
// of that file starts at the first request.
func TestSearchDeclinedDiscardLeavesNoJump(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.CreateFile(ctx, "b.http", searchTwoFiles); err != nil {
		t.Fatal(err)
	}
	hits, _ := mgr.SearchRequests(ctx, "two")
	m := newModel(ctx, mgr, Options{})
	m.selected = "a.http"
	m.dirty = true
	m = searchSelect(t, m, hits[0])
	if m.confirm == nil {
		t.Fatal("opening another file over unsaved edits should ask first")
	}
	m.confirm = nil // declined
	next, _ := m.Update(loadFileCmd(ctx, mgr, "b.http")())
	m = next.(model)
	if got := m.requests.Cursor(); got != 0 {
		t.Fatalf("cursor = %d, want 0", got)
	}
}
