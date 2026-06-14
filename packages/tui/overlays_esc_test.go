package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPromptEscCancels(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatalf("create: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "a.http"
	m.executeCommand("rename-file")
	if m.prompt == nil {
		t.Fatal("rename-file should open a prompt")
	}
	cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.prompt != nil {
		t.Fatal("esc should dismiss the prompt")
	}
	if cmd != nil {
		t.Fatal("esc should not run a command")
	}
	if _, err := mgr.ReadFile(ctx, "a.http"); err != nil {
		t.Fatalf("file should be untouched after cancel: %v", err)
	}
}

func TestSearchEscCloses(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.searchOpen = true
	_ = m.searchInput.Focus()
	cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.searchOpen {
		t.Fatal("esc should close the search overlay")
	}
	if cmd != nil {
		t.Fatal("esc should not return a load command")
	}
	if m.searchInput.Focused() {
		t.Fatal("search input should be blurred after close")
	}
}

func TestHistoryDetailContentNil(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.historyDetail = nil
	out := m.historyDetailContent() // must not panic
	if !strings.Contains(out, "No run selected") {
		t.Fatalf("nil detail should render a safe placeholder, got %q", out)
	}
}
