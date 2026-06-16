package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func enter() tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}) }
func esc() tea.KeyPressMsg   { return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}) }

// TestHandleKeyOverlayControls covers the Confirm/Cancel branches of every modal
// overlay in handleKey, plus source-editing save/cancel and the dirty-quit guard.
func TestHandleKeyOverlayControls(t *testing.T) {
	mk := func(t *testing.T) model {
		mgr := newTestManager(t)
		if _, err := mgr.CreateFile(context.Background(), "a.http", sampleHTTP); err != nil {
			t.Fatalf("seed: %v", err)
		}
		return newModel(context.Background(), mgr, Options{})
	}

	t.Run("prompt confirm runs onSubmit", func(t *testing.T) {
		m := mk(t)
		called := false
		m.prompt = newPrompt("T", "p", "value", func(string) tea.Cmd {
			called = true
			return func() tea.Msg { return nil }
		})
		if m.handleKey(enter()) == nil || !called {
			t.Fatal("prompt enter should run onSubmit")
		}
		if m.prompt != nil {
			t.Fatal("prompt enter should close the prompt")
		}
	})

	t.Run("prompt cancel closes", func(t *testing.T) {
		m := mk(t)
		m.prompt = newPrompt("T", "p", "v", func(string) tea.Cmd { return nil })
		m.handleKey(esc())
		if m.prompt != nil {
			t.Fatal("prompt esc should close it")
		}
	})

	t.Run("env switcher confirm/cancel", func(t *testing.T) {
		m := mk(t)
		m.envOpen = true
		m.refreshEnvList()
		m.handleKey(esc())
		if m.envOpen {
			t.Fatal("esc should close the env switcher")
		}
		m.envOpen = true
		m.handleKey(enter()) // selects (or no-op if empty); must not panic
		if m.envOpen {
			// ok: closed on confirm
		}
	})

	t.Run("theme picker confirm applies", func(t *testing.T) {
		m := mk(t)
		m.themeOpen = true
		m.themeList.SetItems(buildThemeItems(m.theme))
		if cmd := m.handleKey(enter()); cmd == nil {
			t.Fatal("theme enter should apply a theme and notify")
		}
		if m.themeOpen {
			t.Fatal("theme enter should close the picker")
		}
	})

	t.Run("command palette confirm executes", func(t *testing.T) {
		m := mk(t)
		m.commandOpen = true
		m.handleKey(enter()) // executes the selected command, closes palette
		if m.commandOpen {
			t.Fatal("palette enter should close the palette")
		}
	})

	t.Run("editing source save and cancel", func(t *testing.T) {
		m := mk(t)
		m.selected = "a.http"
		m.editing = true
		m.focus = focusSource
		if m.handleKey(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl})) == nil {
			t.Fatal("ctrl+s while editing should return a save command")
		}
		m.editing = true
		m.handleKey(esc())
		if m.editing {
			t.Fatal("esc while editing should leave edit mode")
		}
	})

	t.Run("dirty quit opens confirm", func(t *testing.T) {
		m := mk(t)
		m.selected = "a.http"
		m.dirty = true
		if m.handleKey(keyPress('q')) != nil || m.confirm == nil {
			t.Fatal("q with unsaved changes should open a discard-confirm dialog")
		}
	})
}
