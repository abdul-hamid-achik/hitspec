package tui

import (
	"context"
	"strings"
	"testing"
)

func TestSuggestCopyName(t *testing.T) {
	cases := map[string]string{
		"api/users.http": "api/users-copy.http",
		"a.hitspec":      "a-copy.hitspec",
		"noext":          "noext-copy.http",
	}
	for in, want := range cases {
		if got := suggestCopyName(in); got != want {
			t.Errorf("suggestCopyName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRenameCmd(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatalf("create: %v", err)
	}
	msg := renameFileCmd(ctx, mgr, "a.http", "b.http", "renamed")().(fileRenamedMsg)
	if msg.err != nil {
		t.Fatalf("rename cmd: %v", msg.err)
	}
	if msg.path != "b.http" || msg.action != "renamed" || msg.parsed == nil {
		t.Fatalf("unexpected msg: %+v", msg)
	}
	if _, err := mgr.ReadFile(ctx, "b.http"); err != nil {
		t.Fatalf("renamed file should exist: %v", err)
	}
}

func TestCopyCmd(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatalf("create: %v", err)
	}
	msg := copyFileCmd(ctx, mgr, "a.http", "a-copy.http", "duplicated")().(fileRenamedMsg)
	if msg.err != nil {
		t.Fatalf("copy cmd: %v", msg.err)
	}
	if msg.path != "a-copy.http" || msg.action != "duplicated" {
		t.Fatalf("unexpected msg: %+v", msg)
	}
	// Original still exists.
	if _, err := mgr.ReadFile(ctx, "a.http"); err != nil {
		t.Fatalf("source should still exist: %v", err)
	}
}

func TestRenamePromptWiring(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatalf("create: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "a.http"

	if cmd := m.executeCommand("rename-file"); cmd != nil {
		t.Fatal("rename-file should open a prompt, not return a command")
	}
	if m.prompt == nil {
		t.Fatal("rename-file should open a prompt")
	}
	if m.prompt.input.Value() != "a.http" {
		t.Fatalf("prompt prefilled with %q, want a.http", m.prompt.input.Value())
	}
	// Simulate the user editing the value and submitting (what Enter does).
	msg := m.prompt.onSubmit("renamed.http")().(fileRenamedMsg)
	if msg.err != nil || msg.path != "renamed.http" {
		t.Fatalf("submit -> %+v", msg)
	}
}

func TestRenamePromptRequiresSelection(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.selected = ""
	m.executeCommand("rename-file")
	if m.prompt != nil {
		t.Fatal("rename-file with no selection should not open a prompt")
	}
}

func TestPromptTypingReachesInputNotModal(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.prompt = newPrompt("Rename", "", "x", nil)

	next, _ := m.Update(keyPress('y'))
	nm := next.(model)
	if nm.prompt == nil {
		t.Fatal("typing should not dismiss the prompt")
	}
	if !strings.Contains(nm.prompt.input.Value(), "y") {
		t.Fatalf("typing should reach the input, got %q", nm.prompt.input.Value())
	}
}
