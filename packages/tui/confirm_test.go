package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDeleteOpensConfirmDialogNotImmediate(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "doomed.http", sampleHTTP); err != nil {
		t.Fatalf("create file: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "doomed.http"

	// Requesting delete must NOT delete yet — it opens a confirm dialog.
	if cmd := m.deleteCmd(); cmd != nil {
		t.Fatal("deleteCmd should defer to a confirm dialog, returning no command")
	}
	if m.confirm == nil {
		t.Fatal("deleteCmd should open a confirm dialog")
	}
	if _, err := mgr.ReadFile(ctx, "doomed.http"); err != nil {
		t.Fatalf("file should still exist before confirmation: %v", err)
	}

	// Confirming runs the action and clears the dialog.
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	nm := next.(model)
	if nm.confirm != nil {
		t.Fatal("confirm dialog should close after 'y'")
	}
}

func TestConfirmCancelKeepsState(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.confirm = &confirmState{title: "Delete?", body: "x", action: func() tea.Msg { return simpleMsg{kind: "deleted"} }}

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	if cmd != nil {
		t.Fatal("cancelling confirm should not run the action")
	}
	if next.(model).confirm != nil {
		t.Fatal("confirm dialog should close after 'n'")
	}
}
