package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// TestRunAndDeleteRequireSelection covers the "no file selected" guard arms of
// the run/save/delete command builders: they surface an error and issue no
// command rather than acting on an empty selection.
func TestRunAndDeleteRequireSelection(t *testing.T) {
	builders := map[string]func(m *model) tea.Cmd{
		"runRequestCmd": (*model).runRequestCmd,
		"runFileCmd":    (*model).runFileCmd,
		"saveCmd":       (*model).saveCmd,
		"deleteCmd":     (*model).deleteCmd,
	}
	for name, build := range builders {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.selected = "" // nothing open
		if cmd := build(&m); cmd != nil {
			t.Fatalf("%s with no selection should return nil, got a command", name)
		}
		if m.err == "" {
			t.Fatalf("%s with no selection should set an error", name)
		}
	}
}

// TestRunCommandsWiredWithSelection asserts the run builders produce a command
// (and flip the loading flag) once a file is selected. The returned closures hit
// the network, so they're only checked for non-nil — not executed.
func TestRunCommandsWiredWithSelection(t *testing.T) {
	for _, name := range []string{"request", "file"} {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.selected = "api.http"
		var cmd tea.Cmd
		if name == "request" {
			cmd = m.runRequestCmd()
		} else {
			cmd = m.runFileCmd()
		}
		if cmd == nil {
			t.Fatalf("run %s with a selection should return a command", name)
		}
		if !m.loading {
			t.Fatalf("run %s should set the loading flag", name)
		}
	}
}

// TestCurrentRequestNameFromTable covers both arms: the highlighted row's name,
// and "" (all requests) when the table is empty.
func TestCurrentRequestNameFromTable(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if got := m.currentRequestName(); got != "" {
		t.Fatalf("empty request table should yield \"\", got %q", got)
	}
	m.selected = "api.http"
	m.parsed = &clientmgr.ParsedFileDTO{Requests: []clientmgr.RequestDTO{
		{Name: "Ping", Method: "GET", URL: "https://example.com/api"},
	}}
	m.refreshRequestTables()
	if got := m.currentRequestName(); got != "Ping" {
		t.Fatalf("currentRequestName = %q, want Ping", got)
	}
}

// TestHandleSecondaryKeyNonFormArms covers the non-form key arms of a secondary
// screen: e focuses the form, ctrl+r refreshes, and x issues a stop command.
func TestHandleSecondaryKeyNonFormArms(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenStress)

	if m.handleSecondaryKey(keyPress('e')); !m.formActive {
		t.Fatal("e on a secondary screen should focus the form")
	}

	m2 := newModel(context.Background(), newTestManager(t), Options{})
	m2.setScreen(screenStress)
	if m2.handleSecondaryKey(keyPress('x')) == nil {
		t.Fatal("x on the stress screen should return a stop command")
	}

	m3 := newModel(context.Background(), newTestManager(t), Options{})
	m3.setScreen(screenCookies)
	if m3.handleSecondaryKey(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl})) == nil {
		t.Fatal("ctrl+r on the cookies screen should return a refresh command")
	}
}
