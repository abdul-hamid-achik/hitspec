package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAdHocLine(t *testing.T) {
	cases := []struct {
		in, wantMethod, wantURL string
	}{
		{"GET https://x/y", "GET", "https://x/y"},
		{"post https://x/y", "POST", "https://x/y"},
		{"https://x/y", "GET", "https://x/y"},
		{"  delete   https://x/y  ", "DELETE", "https://x/y"},
		{"", "", ""},
	}
	for _, c := range cases {
		m, u := parseAdHocLine(c.in)
		if m != c.wantMethod || u != c.wantURL {
			t.Errorf("parseAdHocLine(%q) = (%q,%q), want (%q,%q)", c.in, m, u, c.wantMethod, c.wantURL)
		}
	}
}

func TestAdhocCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	msg := adhocCmd(context.Background(), newTestManager(t), "GET "+srv.URL)().(runDoneMsg)
	if msg.err != nil {
		t.Fatalf("adhocCmd: %v", msg.err)
	}
	if msg.result == nil || len(msg.result.Results) == 0 {
		t.Fatalf("expected a result, got %+v", msg.result)
	}
	if rr := msg.result.Results[0]; rr.Response == nil || rr.Response.StatusCode != 200 {
		t.Fatalf("unexpected response: %+v", rr.Response)
	}
}

func TestAdhocCmdRequiresURL(t *testing.T) {
	msg := adhocCmd(context.Background(), newTestManager(t), "   ")().(runDoneMsg)
	if msg.err == nil {
		t.Fatal("adhocCmd with no URL should error")
	}
}

func TestAdhocCommandOpensPrompt(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	if cmd := m.executeCommand("adhoc"); cmd != nil {
		t.Fatal("adhoc should open a prompt, not return a command")
	}
	if m.prompt == nil {
		t.Fatal("adhoc should open the quick-request prompt")
	}
}

func TestRunDoneAdhocFocusesResponse(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.screen = screenStress
	m.focus = focusFiles
	next, _ := m.Update(runDoneMsg{result: sampleResult(), adhoc: true})
	nm := next.(model)
	if nm.screen != screenWorkspace || nm.focus != focusResponse {
		t.Fatalf("ad-hoc run should surface workspace+focusResponse, got screen=%v focus=%v", nm.screen, nm.focus)
	}
}

func TestRunDoneNormalDoesNotStealFocus(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.screen = screenWorkspace
	m.editing = true
	m.focus = focusSource
	next, _ := m.Update(runDoneMsg{result: sampleResult()}) // normal (non-ad-hoc) run
	nm := next.(model)
	if nm.focus != focusSource {
		t.Fatalf("a normal run must not steal focus from an active source edit, got focus=%v", nm.focus)
	}
}
