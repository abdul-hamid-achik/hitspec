package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// searchItem is one request match in the workspace-search overlay.
type searchItem struct {
	file   string
	name   string
	method string
	url    string
	line   int
}

func (i searchItem) FilterValue() string { return i.name + " " + i.url }
func (i searchItem) Title() string {
	name := i.name
	if name == "" {
		name = fmt.Sprintf("line %d", i.line)
	}
	return fmt.Sprintf("%-6s %s", i.method, name)
}
func (i searchItem) Description() string { return i.file + "  " + i.url }

// searchResultMsg carries results for a given query (so stale responses can be
// discarded if the query has moved on).
type searchResultMsg struct {
	query   string
	results []clientmgr.SearchResultDTO
	err     error
}

func searchRequestsCmd(ctx context.Context, mgr *clientmgr.Manager, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := mgr.SearchRequests(ctx, query)
		return searchResultMsg{query: query, results: res, err: err}
	}
}

func (m *model) refreshSearchList() {
	items := make([]list.Item, 0, len(m.searchResults))
	for _, r := range m.searchResults {
		items = append(items, searchItem{file: r.File, name: r.RequestName, method: r.Method, url: r.URL, line: r.Line})
	}
	_ = m.searchList.SetItems(items)
}
