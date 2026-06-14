package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func TestRootViewUsesAltScreenAndRendersWorkspace(t *testing.T) {
	ctx := context.Background()
	mgr := clientmgr.New(
		clientmgr.WithWorkDir(t.TempDir()),
		clientmgr.WithWatch(false),
		clientmgr.WithHistoryDBPath(filepath.Join(t.TempDir(), "history.db")),
	)
	mgr.Start(ctx)
	defer mgr.Close()

	m := newModel(ctx, mgr, Options{})
	m.width = 120
	m.height = 36
	m.workspace = clientmgr.WorkspaceDTO{Root: mgr.Config().WorkDir, Environment: "dev"}
	m.selected = "api.http"
	m.source.SetValue("### Ping\nGET https://example.com\n")
	m.respView.setPlaceholder("No run result yet.")

	view := m.View()
	if !view.AltScreen {
		t.Fatal("expected TUI to use alt screen")
	}
	if !strings.Contains(view.Content, "hitspec studio") {
		t.Fatalf("view content missing title: %q", view.Content)
	}
}

func TestKeybindingsSwitchScreens(t *testing.T) {
	ctx := context.Background()
	mgr := clientmgr.New(
		clientmgr.WithWorkDir(t.TempDir()),
		clientmgr.WithWatch(false),
		clientmgr.WithHistoryDBPath(filepath.Join(t.TempDir(), "history.db")),
	)
	mgr.Start(ctx)
	defer mgr.Close()

	m := newModel(ctx, mgr, Options{})
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "2", Code: '2'}))
	updated := next.(model)
	if updated.screen != screenStress {
		t.Fatalf("screen = %v, want stress", updated.screen)
	}
}

func TestSettingsFormSubmitPersistsEnvironment(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	mgr := clientmgr.New(
		clientmgr.WithWorkDir(dir),
		clientmgr.WithWatch(false),
		clientmgr.WithHistoryDBPath(filepath.Join(dir, "history.db")),
	)
	mgr.Start(ctx)
	defer mgr.Close()

	m := newModel(ctx, mgr, Options{})
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenSettings)
	m.formInputs[0].SetValue("staging")
	m.formInputs[1].SetValue("1500")

	msg := m.submitFormCmd()()
	result, ok := msg.(configMsg)
	if !ok {
		t.Fatalf("submit returned %T, want configMsg", msg)
	}
	if result.err != nil {
		t.Fatalf("submit settings: %v", result.err)
	}
	if result.config.DefaultEnvironment != "staging" {
		t.Fatalf("default env = %q, want staging", result.config.DefaultEnvironment)
	}
}
