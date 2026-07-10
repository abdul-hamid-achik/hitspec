package tui

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// TestHandleKey_FilteringNotHijackedByGlobalHotkeys guards the regression where
// typing into the files-list filter was hijacked by global hotkeys: "q" quit the
// app, "D" deleted a file, and digits 1-9 switched screens while a filter was
// being typed. While the files list is filtering, those keys must go to the
// filter input instead.
func TestHandleKey_FilteringNotHijackedByGlobalHotkeys(t *testing.T) {
	mk := func(t *testing.T) model {
		mgr := clientmgr.New(clientmgr.WithWorkDir(t.TempDir()), clientmgr.WithWatch(false))
		mgr.Start(context.Background())
		t.Cleanup(func() { _ = mgr.Close() })
		if _, err := mgr.CreateFile(context.Background(), "a.http", sampleHTTP); err != nil {
			t.Fatalf("seed: %v", err)
		}
		m := newModel(context.Background(), mgr, Options{})
		m.screen = screenWorkspace
		m.focus = focusFiles
		// Put the files list into the filtering state (as if the user pressed /
		// and is typing).
		m.filesList.SetFilterText("a")
		m.filesList.SetFilterState(list.Filtering)
		return m
	}

	t.Run("q does not quit", func(t *testing.T) {
		m := mk(t)
		cmd := m.handleKey(keyPress('q'))
		if isQuit(cmd) {
			t.Fatal("q while filtering must go to the filter, not quit the app")
		}
		if m.filesList.FilterState() != list.Filtering {
			t.Fatal("filter state should still be Filtering after typing q")
		}
	})

	t.Run("digit does not switch screen", func(t *testing.T) {
		m := mk(t)
		// "2" switches to the stress screen when not filtering.
		m.handleKey(keyPress('2'))
		if m.screen != screenWorkspace {
			t.Fatalf("digit while filtering switched screen to %v", m.screen)
		}
	})
}

// isQuit reports whether cmd, when executed, produces a tea.QuitMsg.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}
