package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
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
