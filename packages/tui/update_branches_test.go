package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// TestFilesLoadedMsgBranches covers the error arm and the stale-selection clear:
// when a reload drops the open file, the selection/source are reset.
func TestFilesLoadedMsgBranches(t *testing.T) {
	// error arm
	m := newModel(context.Background(), newTestManager(t), Options{})
	next, _ := m.Update(filesLoadedMsg{err: fmt.Errorf("disk gone")})
	if next.(model).err == "" {
		t.Fatal("filesLoadedMsg error should set m.err")
	}

	// stale-selection clear: selected file is no longer present after reload
	m = newModel(context.Background(), newTestManager(t), Options{})
	m.selected = "ghost.http"
	m.source.SetValue("stale")
	next, _ = m.Update(filesLoadedMsg{
		workspace: clientmgr.WorkspaceDTO{},
		files:     []clientmgr.FileInfoDTO{{RelativePath: "other.http", Name: "other.http"}},
	})
	nm := next.(model)
	if nm.selected == "ghost.http" {
		t.Fatal("a reload that drops the open file should clear the selection")
	}
}

// TestMediumMainFocuses renders the medium-width workspace at each focus so the
// content-switch arms of mediumMain are exercised.
func TestMediumMainFocuses(t *testing.T) {
	m := goldenModel(t, 100, 30) // medium layout (78–120 cols)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "api.http", Name: "api.http", RequestCount: 1}}
	m.selected = "api.http"
	m.parsed = &clientmgr.ParsedFileDTO{Requests: []clientmgr.RequestDTO{{Name: "Ping", Method: "GET", URL: "https://x/y"}}}
	m.refreshFileList()
	m.refreshRequestTables()
	m.lastResult = sampleResult()
	m.refreshResultViews()

	for _, f := range []focusPane{focusRequests, focusSource, focusResponse} {
		m.focus = f
		if strings.TrimSpace(plain(m.workspaceView())) == "" {
			t.Fatalf("medium workspace rendered empty at focus %v", f)
		}
	}
}

// TestHandleHistoryKeyForwarding covers the list-navigation fall-through (arrow
// keys reach the list) and the detail-mode refresh/scroll arms.
func TestHandleHistoryKeyForwarding(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenHistory)
	m.history = sampleHistory()
	m.refreshHistoryList()

	// an arrow key in the list view is forwarded to the list (no panic)
	m.handleHistoryKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))

	// detail mode: a scroll key falls through to the viewport, esc returns
	m.historyDetailMode = true
	m.handleHistoryKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m.handleHistoryKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.historyDetailMode {
		t.Fatal("esc should leave history detail mode")
	}
}
