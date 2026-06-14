package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

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
