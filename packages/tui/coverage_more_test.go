package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func TestHighlightBranches(t *testing.T) {
	// color off → unchanged
	if got := highlight(`{"a":1}`, "json", false); got != `{"a":1}` {
		t.Fatalf("color-off should be unchanged, got %q", got)
	}
	// empty → unchanged
	if got := highlight("   ", "json", true); got != "   " {
		t.Fatalf("blank should be unchanged, got %q", got)
	}
	// color on with a known lexer → still carries the content (ANSI stripped)
	got := plain(highlight(`{"hello":"world"}`, "json", true))
	if !strings.Contains(got, "hello") {
		t.Fatalf("highlighted JSON lost its content: %q", got)
	}
	// unknown lexer hint falls back to analysis, still returns the content
	got = plain(highlight("GET https://x/y\n", "no-such-lexer", true))
	if !strings.Contains(got, "https://x/y") {
		t.Fatalf("highlight with unknown lexer lost content: %q", got)
	}
}

func TestFirstNonSpace(t *testing.T) {
	if firstNonSpace("   \n\t x") != 'x' {
		t.Fatal("firstNonSpace should skip whitespace")
	}
	if firstNonSpace("   ") != 0 {
		t.Fatal("all-whitespace should yield 0")
	}
}

func TestStatusbarStates(t *testing.T) {
	m := goldenModel(t, 100, 30)

	// a long error is truncated to fit, never overflowing the bar width
	m.err = strings.Repeat("boom ", 80)
	if w := lipgloss.Width(plain(m.statusbar())); w > 100 {
		t.Fatalf("statusbar overflowed width: %d", w)
	}

	// loading prepends the spinner; progress is appended
	m.err = ""
	m.loading = true
	m.status = "working"
	m.progress = clientmgr.RequestProgress{RequestName: "ping", Index: 1, Total: 3}
	out := plain(m.statusbar())
	if !strings.Contains(out, "working") || !strings.Contains(out, "1/3") {
		t.Fatalf("statusbar missing status/progress: %q", out)
	}
}

func TestHandleKeyWorkspaceBranches(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "a.http", sampleHTTP); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// enter on a focused file row loads it
	m := newModel(ctx, mgr, Options{})
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "a.http", Name: "a.http", RequestCount: 1}}
	m.refreshFileList()
	m.focus = focusFiles
	if cmd := m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})); cmd == nil {
		t.Fatal("enter on a file row should return a load command")
	}

	// g on an empty workspace scaffolds a sample
	m2 := newModel(ctx, newTestManager(t), Options{})
	m2.files = nil
	if cmd := m2.handleKey(keyPress('g')); cmd == nil {
		t.Fatal("g on an empty workspace should scaffold")
	}

	// an arrow key in files focus is forwarded to the list (no panic, may be nil)
	m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	// switch to requests focus and forward there too
	m.focus = focusRequests
	m.handleKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
}

func TestFileopsCmds(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "orig.http", sampleHTTP); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// rename success
	msg := renameFileCmd(ctx, mgr, "orig.http", "moved.http", "renamed")()
	rm, ok := msg.(fileRenamedMsg)
	if !ok || rm.err != nil || rm.path != "moved.http" || rm.action != "renamed" {
		t.Fatalf("renameFileCmd success wrong: %#v", msg)
	}

	// duplicate (copy) success
	msg = copyFileCmd(ctx, mgr, "moved.http", "copy.http", "duplicated")()
	cm, ok := msg.(fileRenamedMsg)
	if !ok || cm.err != nil || cm.action != "duplicated" {
		t.Fatalf("copyFileCmd success wrong: %#v", msg)
	}

	// rename of a missing file surfaces an error
	msg = renameFileCmd(ctx, mgr, "ghost.http", "x.http", "renamed")()
	if rm, ok := msg.(fileRenamedMsg); !ok || rm.err == nil {
		t.Fatalf("renameFileCmd on a missing file should error: %#v", msg)
	}
}

func TestBodyTabTruncatesHugeBody(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.tab = respBody
	huge := strings.Repeat("x", maxResponseBodyRender+5000)
	rv.setResult(&clientmgr.RunResultDTO{Results: []clientmgr.RequestResultDTO{{
		Name: "big", Response: &clientmgr.HTTPResponseDTO{StatusCode: 200, Status: "200 OK", Body: huge},
	}}})
	if !strings.Contains(plain(rv.tabContent()), "truncated") {
		t.Fatal("an oversized body should be marked truncated")
	}
}
