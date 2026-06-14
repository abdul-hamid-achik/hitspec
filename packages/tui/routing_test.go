package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// keyPress is a small helper for sending a single printable rune.
func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: string(r), Code: r})
}

// TestEditKeyDoesNotLeakIntoSource guards the fall-through bug where the 'e' that
// entered edit mode was also delivered to the textarea.
func TestEditKeyDoesNotLeakIntoSource(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.selected = "api.http"
	const src = "### Ping\nGET https://example.com\n"
	m.source.SetValue(src)

	next, _ := m.Update(keyPress('e')) // Edit binding
	nm := next.(model)
	if !nm.editing {
		t.Fatal("'e' should enter edit mode")
	}
	if nm.source.Value() != src {
		t.Fatalf("'e' leaked into source: %q", nm.source.Value())
	}
	if nm.dirty {
		t.Fatal("entering edit mode should not mark the buffer dirty")
	}
}

// TestEditKeyDoesNotLeakIntoForm guards the same bug on the secondary form path.
func TestEditKeyDoesNotLeakIntoForm(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenCookies)
	domain := m.formInputs[0].Value()

	next, _ := m.Update(keyPress('e')) // focus the form
	nm := next.(model)
	if !nm.formActive {
		t.Fatal("'e' should focus the form")
	}
	if got := nm.formInputs[0].Value(); got != domain {
		t.Fatalf("'e' leaked into form field: %q (want %q)", got, domain)
	}
}

// TestConfirmModalCapturesKeys ensures stray keys don't reach background widgets
// while a confirm dialog is open.
func TestConfirmModalCapturesKeys(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.lastResult = sampleResult()
	m.refreshResultViews()
	m.focus = focusResponse
	m.confirm = &confirmState{title: "Delete?", body: "x", action: func() tea.Msg { return simpleMsg{kind: "x"} }}
	startTab := m.respView.tab

	// A tab-switch key must be swallowed by the modal, not switch the hidden tab.
	next, _ := m.Update(keyPress(']'))
	nm := next.(model)
	if nm.respView.tab != startTab {
		t.Fatal("']' leaked to the response viewer behind the confirm modal")
	}
	if nm.confirm == nil {
		t.Fatal("a non-answer key should not dismiss the confirm dialog")
	}
}

// TestEnvSwitcherNavigationStillWorks ensures the transitioned guard does not
// break navigation inside an open overlay (regression guard for the fix).
func TestEnvSwitcherNavigationStillWorks(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.envs = []clientmgr.EnvironmentDTO{
		{Name: "dev"}, {Name: "staging"}, {Name: "prod"},
	}
	m.envOpen = true
	m.refreshEnvList()
	m.envList.Select(0)

	next, _ := m.Update(keyPress('j')) // vim-down in the list
	nm := next.(model)
	if nm.envList.Index() == 0 {
		t.Fatal("'j' should still navigate the open env switcher")
	}
}

// TestEditModeTypingStillWorks ensures normal typing reaches the textarea once
// edit mode is active (i.e. the guard only suppresses the transition key).
func TestEditModeTypingStillWorks(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.selected = "api.http"
	m.source.SetValue("")
	m.editing = true
	m.focus = focusSource
	_ = m.source.Focus()

	next, _ := m.Update(keyPress('x'))
	nm := next.(model)
	if !strings.Contains(nm.source.Value(), "x") {
		t.Fatalf("typing in edit mode should reach the textarea, got %q", nm.source.Value())
	}
}
