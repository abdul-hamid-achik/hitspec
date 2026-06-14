package tui

import (
	"context"
	"testing"
)

func TestFocusCyclingWraps(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	for _, want := range []focusPane{focusRequests, focusSource, focusResponse, focusFiles} {
		m.nextFocus()
		if m.focus != want {
			t.Fatalf("nextFocus -> %v, want %v", m.focus, want)
		}
	}
	m.prevFocus() // focusFiles wraps back to focusResponse
	if m.focus != focusResponse {
		t.Fatalf("prevFocus wrap -> %v, want focusResponse", m.focus)
	}
}

func TestMoveFormFocusWraps(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenCookies)
	n := len(m.formInputs)
	if n == 0 {
		t.Fatal("cookies screen should have form inputs")
	}
	m.formActive = true
	m.formFocus = 0
	m.moveFormFocus(-1)
	if m.formFocus != n-1 {
		t.Fatalf("moveFormFocus(-1) -> %d, want %d", m.formFocus, n-1)
	}
	m.moveFormFocus(1)
	if m.formFocus != 0 {
		t.Fatalf("moveFormFocus(1) -> %d, want 0", m.formFocus)
	}
}

func TestExecuteCommandCopyFormats(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "sample.http", sampleHTTP); err != nil {
		t.Fatalf("create file: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "sample.http"

	for _, id := range []string{"copy-httpie", "copy-python", "copy-fetch", "copy-go"} {
		cm, ok := m.executeCommand(id)().(copyMsg)
		if !ok {
			t.Fatalf("%s did not emit copyMsg", id)
		}
		if cm.err != nil {
			t.Fatalf("%s errored: %v", id, cm.err)
		}
		if cm.content == "" {
			t.Fatalf("%s produced empty content", id)
		}
	}
}

func TestExecuteCommandScreenFallthrough(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.executeCommand("mock")
	if m.screen != screenMock {
		t.Fatalf("executeCommand(mock) -> screen %v, want screenMock", m.screen)
	}
}

func TestExecuteCommandEnvSwitch(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.executeCommand("env-switch")
	if !m.envOpen {
		t.Fatal("env-switch should open the environment switcher")
	}
}

func TestExecuteCommandStartActionsReturnCommands(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	// Don't invoke the closures (they'd start real servers) — just assert wiring.
	if m.executeCommand("stress-start") == nil {
		t.Fatal("stress-start should return a command")
	}
	if m.executeCommand("mock-start") == nil {
		t.Fatal("mock-start should return a command")
	}
}

func TestStopScreenCmdStress(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.screen = screenStress
	cm, ok := m.stopScreenCmd()().(simpleMsg)
	if !ok {
		t.Fatal("stopScreenCmd(stress) should emit simpleMsg")
	}
	if cm.kind != "stress stopping" {
		t.Fatalf("kind = %q, want 'stress stopping'", cm.kind)
	}
}

func TestRecordExportSecondaryKey(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.setScreen(screenRecord)
	cmd := m.handleSecondaryKey(keyPress('E'))
	if cmd == nil {
		t.Fatal("'E' on the record screen should return an export command")
	}
	if _, ok := cmd().(previewMsg); !ok {
		t.Fatalf("record export -> %T, want previewMsg", cmd())
	}
}
