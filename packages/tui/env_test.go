package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func TestRefreshEnvListMarksActive(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.workspace.Environment = "staging"
	m.envs = []clientmgr.EnvironmentDTO{
		{Name: "dev", Variables: map[string]any{"a": 1}},
		{Name: "staging", Variables: map[string]any{"a": 1, "b": 2}},
	}
	m.refreshEnvList()

	if got := len(m.envList.Items()); got != 2 {
		t.Fatalf("want 2 env items, got %d", got)
	}
	sel, ok := m.envList.SelectedItem().(envItem)
	if !ok || sel.name != "staging" || !sel.active {
		t.Fatalf("active env not pre-selected: %+v", m.envList.SelectedItem())
	}
}

func TestEnvSwitcherOpensOnCtrlE(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	if !next.(model).envOpen {
		t.Fatal("ctrl+e should open the environment switcher")
	}
}

func TestEnvSelectedMsgUpdatesWorkspace(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	next, _ := m.Update(envSelectedMsg{
		name:      "prod",
		config:    clientmgr.ConfigDTO{DefaultEnvironment: "prod"},
		workspace: clientmgr.WorkspaceDTO{Environment: "prod"},
	})
	nm := next.(model)
	if nm.workspace.Environment != "prod" {
		t.Fatalf("workspace env = %q, want prod", nm.workspace.Environment)
	}
	found := false
	for _, tt := range nm.toasts.items {
		if tt.severity == toastSuccess {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a success toast after switching environments")
	}
}

func TestSelectEnvCmd(t *testing.T) {
	msg := selectEnvCmd(context.Background(), newTestManager(t), "dev")()
	em, ok := msg.(envSelectedMsg)
	if !ok {
		t.Fatalf("want envSelectedMsg, got %T", msg)
	}
	if em.err != nil {
		t.Fatalf("selectEnvCmd(dev) errored: %v", em.err)
	}
	if em.name != "dev" {
		t.Fatalf("name = %q, want dev", em.name)
	}
}
