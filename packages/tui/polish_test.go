package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// An error in the status bar must survive incidental keypresses (opening help,
// navigating) — it is the only persistent view of the error.
func TestErrorPersistsAcrossKeypress(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.err = "boom: connection refused"
	m.handleKey(keyPress('?')) // open help — must NOT clear the error
	if m.err == "" {
		t.Fatal("error was cleared by pressing ? (help)")
	}
	// Switching screens starts fresh and clears it.
	m.setScreen(screenStress)
	if m.err != "" {
		t.Fatalf("error not cleared on screen switch: %q", m.err)
	}
}

// A successful result message must clear a stale error — not just the four
// handlers fixed first, but every result handler (regression from review w1gbxcu13).
func TestSuccessfulResultMessagesClearError(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.Msg
	}{
		{"history", historyMsg{history: clientmgr.HistoryListDTO{}}},
		{"cookies", cookiesMsg{cookies: nil}},
		{"config", configMsg{config: clientmgr.ConfigDTO{}, envs: nil}},
		{"contract", contractMsg{results: nil}},
		{"preview", previewMsg{title: "done", content: "ok"}},
		{"copy", copyMsg{title: "copied"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := goldenModel(t, 100, 30)
			m.err = "stale: previous failure"
			next, _ := m.Update(tc.msg)
			if got := next.(model).err; got != "" {
				t.Fatalf("%s success did not clear m.err: %q", tc.name, got)
			}
		})
	}
}

func TestQuitConfirmsWhenDirty(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.selected = "api.http"

	// Clean: q quits immediately.
	cmd := m.handleKey(keyPress('q'))
	if cmd == nil {
		t.Fatal("clean quit returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("clean quit did not produce a QuitMsg")
	}

	// Dirty: q opens a confirm modal instead of quitting.
	m.dirty = true
	if cmd := m.handleKey(keyPress('q')); cmd != nil {
		t.Fatal("dirty quit should not return a quit command")
	}
	if m.confirm == nil {
		t.Fatal("dirty quit should open a confirm modal")
	}
}

func TestCtrlCAlwaysQuits(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.dirty = true // even with unsaved changes, ctrl+c is a hard interrupt
	cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("ctrl+c returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not produce a QuitMsg")
	}
	if m.confirm != nil {
		t.Fatal("ctrl+c should not open a confirm modal")
	}
}

func TestQuitWhileEditingDoesNotQuit(t *testing.T) {
	// While editing source, q is a literal character (e.g. typing "query"), not a
	// quit. The editing key block swallows it before the Quit case, and Update
	// forwards it to the textarea — so handleKey returns nil and stays in edit mode.
	m := goldenModel(t, 100, 30)
	m.selected = "api.http"
	m.focus = focusSource
	m.editing = true
	if cmd := m.handleKey(keyPress('q')); cmd != nil {
		t.Fatal("q while editing must not quit")
	}
	if !m.editing {
		t.Fatal("q while editing must stay in edit mode (q is typed, not a command)")
	}
}

func TestResizeGuardsSmallTerminals(t *testing.T) {
	// Tiny/edge sizes must not produce negative viewport heights or panic.
	for _, h := range []int{1, 6, 12, 14, 20} {
		m := goldenModel(t, 80, h)
		if m.source.Height() < 1 {
			t.Fatalf("source height %d < 1 at terminal height %d", m.source.Height(), h)
		}
		if got := m.View().Content; got == "" {
			t.Fatalf("empty render at terminal height %d", h)
		}
	}
}

func TestSecondaryEmptyStatesAreActionable(t *testing.T) {
	m := goldenModel(t, 100, 30)
	if got := m.mockContent(); !strings.Contains(got, "No mock server running") || !strings.Contains(got, "enter") {
		t.Fatalf("mock empty state not actionable: %q", got)
	}
	if got := m.recordContent(); !strings.Contains(got, "No recording proxy running") || !strings.Contains(got, "enter") {
		t.Fatalf("record empty state not actionable: %q", got)
	}
}

func TestNewInputClampsToWidth(t *testing.T) {
	m := goldenModel(t, 50, 24) // narrow
	in := m.newInput("base URL", "", 64)
	if in.Width() > 50-8 {
		t.Fatalf("input width %d not clamped to narrow terminal (50)", in.Width())
	}
	wide := goldenModel(t, 130, 40)
	in2 := wide.newInput("base URL", "", 44)
	if in2.Width() != 44 {
		t.Fatalf("input width %d should be unclamped on a wide terminal, want 44", in2.Width())
	}
}

func TestSearchOverlayShowsCountAndHint(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.searchOpen = true
	m.searchInput.SetValue("users")
	m.searchResults = []clientmgr.SearchResultDTO{
		{File: "api.http", RequestName: "List users", Method: "GET", URL: "https://x/users"},
	}
	m.refreshSearchList()
	out := plain(m.View().Content)
	if !strings.Contains(out, "1 results") {
		t.Fatalf("search overlay missing result count: %q", out)
	}
	if !strings.Contains(out, "esc cancel") {
		t.Fatalf("search overlay missing footer hint: %q", out)
	}
}
