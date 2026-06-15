package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// TestHandleEventStressAndMock covers the live-update event branches: a
// stress_update refreshes the cached status (and the preview while on the stress
// screen), a terminal update marks the run stopped, and a mock_request pulls the
// mock status without panicking.
func TestHandleEventStressAndMock(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.width, m.height = 100, 30
	m.resize()
	m.setScreen(screenStress)

	m.handleEvent(clientmgr.Event{Type: "stress_update", Payload: clientmgr.StressMetrics{
		Running: true, Elapsed: 2.5,
		Stats: clientmgr.StressStatsDTO{Total: 100, Success: 98, Errors: 2, RPS: 40, P95Ms: 30, ActiveVUs: 5},
	}})
	if m.stress.Stats == nil || !m.stress.Running {
		t.Fatalf("stress_update did not update status: %+v", m.stress)
	}
	if !strings.Contains(plain(m.preview.View()), "Running") {
		t.Fatalf("stress preview not refreshed:\n%s", plain(m.preview.View()))
	}

	// A terminal (Running:false) update marks the run stopped and fetches the
	// final result — must not panic against the empty test manager.
	m.handleEvent(clientmgr.Event{Type: "stress_update", Payload: clientmgr.StressMetrics{
		Running: false, Completed: true, Stats: clientmgr.StressStatsDTO{Total: 100},
	}})
	if m.stress.Running {
		t.Fatal("stress_update Running:false should mark the run stopped")
	}

	// mock_request refreshes the cached mock status while on the mock screen.
	m.setScreen(screenMock)
	m.handleEvent(clientmgr.Event{Type: "mock_request"})
}

// TestFormEditCapturesGlobalKeys guards the bug where typing into a focused
// secondary-screen form field still triggered global key bindings — e.g. a "9"
// jumped to the settings screen instead of landing in the field, making numeric
// fields (rate, port, vus) impossible to fill.
func TestFormEditCapturesGlobalKeys(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.width, m.height = 100, 30
	m.resize()
	m.setScreen(screenStress)
	m.focusForm(true) // enter edit mode on the first field (duration)

	before := m.formInputs[0].Value()
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "9", Code: '9'}))
	nm := next.(model)
	if nm.screen != screenStress {
		t.Fatalf("typing '9' into a focused form field switched screens to %v", nm.screen)
	}
	if nm.formInputs[0].Value() == before {
		t.Fatalf("typing '9' did not reach the focused field (value still %q)", before)
	}
}

func TestStressResultContent(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.stressResult = &clientmgr.StressResultDTO{
		DurationMs:  30000,
		Total:       1000,
		Success:     990,
		Errors:      10,
		RPS:         33.3,
		SuccessRate: 0.99,
		ErrorRate:   0.01,
		P50Ms:       12,
		P95Ms:       45,
		P99Ms:       80,
		MinMs:       5,
		MaxMs:       120,
		MeanMs:      15,
		Breakdown: []clientmgr.StressRequestBreakdownDTO{
			{Name: "login", Total: 500, Success: 495, Errors: 5, P95Ms: 40},
		},
	}
	// Stats nil → stressContent falls back to the completed-result view.
	out := m.stressContent()
	for _, want := range []string{"Last result", "990", "p95", "login"} {
		if !strings.Contains(out, want) {
			t.Errorf("stress result content missing %q in:\n%s", want, out)
		}
	}
}

func TestStressContentEmpty(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if !strings.Contains(m.stressContent(), "No stress test running") {
		t.Fatalf("expected empty stress hint, got %q", m.stressContent())
	}
}

func TestLoadScreenStateNoPanic(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	for _, s := range []screen{screenStress, screenMock, screenRecord} {
		m.screen = s
		m.loadScreenState() // nothing running — must not panic
	}
	if m.mock.Running || m.record.Running {
		t.Fatal("nothing should be running in a fresh manager")
	}
}

func TestSecondaryViewRendersPreview(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.width, m.height = 100, 30
	m.screen = screenStress
	m.preview.SetContent("STRESSMARKER")
	if !strings.Contains(m.secondaryView(), "STRESSMARKER") {
		t.Fatalf("secondaryView should render the preview content")
	}
}

func TestClearCommandsOpenConfirm(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})

	// Bulk-destructive actions defer to a confirm dialog (no immediate clear).
	if cmd := m.executeCommand("record-clear"); cmd != nil {
		t.Fatal("record-clear should defer to a confirm dialog, not run immediately")
	}
	if m.confirm == nil {
		t.Fatal("record-clear should open a confirm dialog")
	}
	rc, ok := m.confirm.action().(simpleMsg)
	if !ok || rc.kind != "recordings cleared" {
		t.Fatalf("record-clear confirm action -> %#v", rc)
	}

	m.confirm = nil
	if cmd := m.executeCommand("history-clear"); cmd != nil {
		t.Fatal("history-clear should defer to a confirm dialog")
	}
	if m.confirm == nil {
		t.Fatal("history-clear should open a confirm dialog")
	}
	if _, ok := m.confirm.action().(historyMsg); !ok {
		t.Fatal("history-clear confirm action should emit a historyMsg")
	}
}
