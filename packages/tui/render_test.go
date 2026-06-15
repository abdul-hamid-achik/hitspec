package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func sampleResult() *clientmgr.RunResultDTO {
	return &clientmgr.RunResultDTO{
		File:   "api.http",
		Passed: 1,
		Results: []clientmgr.RequestResultDTO{{
			Name:     "Ping",
			Passed:   true,
			Duration: 12,
			Request:  &clientmgr.HTTPRequestDTO{Method: "GET", URL: "https://example.com/api"},
			Response: &clientmgr.HTTPResponseDTO{
				StatusCode: 200,
				Status:     "200 OK",
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"hello":"world"}`,
				Size:       17,
			},
			Assertions: []clientmgr.AssertionResultDTO{
				{Subject: "status", Operator: "==", Expected: 200, Passed: true},
			},
			Captures: map[string]any{"id": "abc"},
		}},
	}
}

func TestResponseViewerBodyTab(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setResult(sampleResult())
	out := rv.tabContent()
	for _, want := range []string{"200 OK", `"hello": "world"`} {
		if !strings.Contains(out, want) {
			t.Errorf("body tab missing %q in:\n%s", want, out)
		}
	}
}

func TestResponseViewerTabsCycle(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setResult(sampleResult())

	rv.tab = respAssertions
	if !strings.Contains(rv.tabContent(), "status") {
		t.Errorf("assertions tab missing assertion subject:\n%s", rv.tabContent())
	}
	rv.tab = respCaptures
	if !strings.Contains(rv.tabContent(), "id") {
		t.Errorf("captures tab missing captured key:\n%s", rv.tabContent())
	}
	rv.tab = respTiming
	if !strings.Contains(rv.tabContent(), "Ping") {
		t.Errorf("timing tab missing request name:\n%s", rv.tabContent())
	}
	rv.tab = respHeaders
	if !strings.Contains(rv.tabContent(), "Content-Type") {
		t.Errorf("headers tab missing header:\n%s", rv.tabContent())
	}

	// nextTab/prevTab wrap correctly.
	rv.tab = respCaptures
	rv.nextTab()
	if rv.tab != respBody {
		t.Errorf("nextTab from last should wrap to Body, got %v", rv.tab)
	}
	rv.prevTab()
	if rv.tab != respCaptures {
		t.Errorf("prevTab from Body should wrap to Captures, got %v", rv.tab)
	}
}

// TestResponseViewerTabBarFits proves the tab bar adapts to the pane width: it
// shows every label when there's room, and collapses to the active tab plus a
// position indicator (never sliced mid-word) when the pane is too narrow.
func TestResponseViewerTabBarFits(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setResult(sampleResult())

	rv.setSize(80, 20) // wide: full strip fits
	wide := plain(rv.tabBar())
	for _, name := range responseTabNames {
		if !strings.Contains(wide, name) {
			t.Fatalf("wide tab bar dropped %q:\n%s", name, wide)
		}
	}
	if lipgloss.Width(rv.tabBar()) > rv.width {
		t.Fatalf("wide tab bar overflows pane width %d:\n%s", rv.width, wide)
	}

	rv.tab = respCaptures
	rv.setSize(16, 20) // narrow: must collapse and stay within the pane
	narrow := rv.tabBar()
	if lipgloss.Width(narrow) > rv.width {
		t.Fatalf("narrow tab bar overflows pane width %d: %q", rv.width, plain(narrow))
	}
	plainNarrow := plain(narrow)
	if !strings.Contains(plainNarrow, "Captures") || !strings.Contains(plainNarrow, "5/5") {
		t.Fatalf("collapsed tab bar should show the active tab and position, got %q", plainNarrow)
	}
}

func TestResponseViewerPlaceholder(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setPlaceholder("file summary here")
	if rv.hasResult() {
		t.Fatal("placeholder should clear the result")
	}
	if !strings.Contains(rv.view(), "file summary here") {
		t.Fatalf("placeholder not shown: %q", rv.view())
	}
}

func TestCopyResponseCommand(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.lastResult = sampleResult()

	cmd := m.executeCommand("copy-response")
	if cmd == nil {
		t.Fatal("copy-response returned no command")
	}
	cm, ok := cmd().(copyMsg)
	if !ok {
		t.Fatalf("expected copyMsg, got %T", cmd())
	}
	if cm.err != nil {
		t.Fatalf("copy-response errored: %v", cm.err)
	}
	if !strings.Contains(cm.title, "response body") {
		t.Fatalf("unexpected copy title: %q", cm.title)
	}
}

func TestCopyAsCurlCommand(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "sample.http", sampleHTTP); err != nil {
		t.Fatalf("create file: %v", err)
	}
	m := newModel(ctx, mgr, Options{})
	m.selected = "sample.http"

	cm := m.executeCommand("copy-curl")().(copyMsg)
	if cm.err != nil {
		t.Fatalf("copy-curl errored: %v", cm.err)
	}
	if !strings.Contains(cm.content, "curl") {
		t.Fatalf("copy-curl content missing curl: %q", cm.content)
	}
}

func TestScreenSwitchKeysTableDriven(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		key  string
		want screen
	}{
		{"1", screenWorkspace},
		{"2", screenStress},
		{"3", screenMock},
		{"4", screenContract},
		{"5", screenRecord},
		{"6", screenHistory},
		{"7", screenImport},
		{"8", screenCookies},
		{"9", screenSettings},
	}
	for _, c := range cases {
		m := newModel(ctx, newTestManager(t), Options{})
		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: c.key, Code: rune(c.key[0])}))
		if got := next.(model).screen; got != c.want {
			t.Errorf("key %q -> screen %v, want %v", c.key, got, c.want)
		}
	}
}
