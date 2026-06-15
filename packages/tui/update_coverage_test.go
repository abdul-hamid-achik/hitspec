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

// TestSubmitFormCmdSafeScreens executes the submit closures that don't bind
// ports or hit the network (cookies → local store, import → curl parse) and
// asserts the rest are at least wired up (the closures would start real
// servers, so they're only checked for non-nil).
func TestSubmitFormCmdSafeScreens(t *testing.T) {
	t.Run("cookies", func(t *testing.T) {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.setScreen(screenCookies)
		m.formInputs[3].SetValue("tok") // value
		msg := m.submitFormCmd()()
		cm, ok := msg.(cookiesMsg)
		if !ok {
			t.Fatalf("cookies submit -> %T, want cookiesMsg", msg)
		}
		if cm.err != nil {
			t.Fatalf("cookies submit errored: %v", cm.err)
		}
	})

	t.Run("import", func(t *testing.T) {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.setScreen(screenImport)
		msg := m.submitFormCmd()() // defaults: curl, "curl https://example.com"
		im, ok := msg.(importMsg)
		if !ok {
			t.Fatalf("import submit -> %T, want importMsg", msg)
		}
		if im.err != nil {
			t.Fatalf("import submit errored: %v", im.err)
		}
	})

	// Server/network submits: assert wiring only.
	for _, s := range []screen{screenStress, screenMock, screenContract, screenRecord} {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.selected = "api.http"
		m.setScreen(s)
		if m.submitFormCmd() == nil {
			t.Fatalf("submitFormCmd for screen %v returned nil", s)
		}
	}
}

// TestExecuteCommandOpenAndSwitch covers the palette command ids that open an
// overlay, switch a screen, or build an export command — the cheap, side-effect-
// free branches of executeCommand.
func TestExecuteCommandOpenAndSwitch(t *testing.T) {
	screenSwitch := map[string]screen{
		"history":  screenHistory,
		"cookies":  screenCookies,
		"settings": screenSettings,
	}
	for id, want := range screenSwitch {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.executeCommand(id)
		if m.screen != want {
			t.Fatalf("executeCommand(%q) -> screen %v, want %v", id, m.screen, want)
		}
	}

	m := newModel(context.Background(), newTestManager(t), Options{})
	if m.executeCommand("theme"); !m.themeOpen {
		t.Fatal("theme should open the theme picker")
	}
	m = newModel(context.Background(), newTestManager(t), Options{})
	if m.executeCommand("search"); !m.searchOpen {
		t.Fatal("search should open the search overlay")
	}
	m = newModel(context.Background(), newTestManager(t), Options{})
	m.selected = "api.http"
	if m.executeCommand("duplicate-file"); m.prompt == nil {
		t.Fatal("duplicate-file should open a path prompt")
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

func TestStopScreenCmdMockAndRecord(t *testing.T) {
	t.Run("mock", func(t *testing.T) {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.screen = screenMock
		cmd := m.stopScreenCmd()
		if cmd == nil {
			t.Fatal("stopScreenCmd(mock) returned nil")
		}
		if _, ok := cmd().(simpleMsg); !ok {
			t.Fatalf("stopScreenCmd(mock) -> %T, want simpleMsg", cmd())
		}
	})
	t.Run("record", func(t *testing.T) {
		m := newModel(context.Background(), newTestManager(t), Options{})
		m.screen = screenRecord
		cm, ok := m.stopScreenCmd()().(simpleMsg)
		if !ok {
			t.Fatalf("stopScreenCmd(record) did not emit simpleMsg")
		}
		if cm.kind != "recording proxy stopping" {
			t.Fatalf("record stop kind = %q", cm.kind)
		}
	})
	// A screen with no stoppable action returns nil.
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.screen = screenWorkspace
	if m.stopScreenCmd() != nil {
		t.Fatal("stopScreenCmd(workspace) should be nil")
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
