package clientmgr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteAdHocPostWithHeadersAndBody(t *testing.T) {
	var gotMethod, gotHeader, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Test")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := newTestManager(t)
	_, err := m.ExecuteAdHoc(context.Background(), AdHocReq{
		Method:  "POST",
		URL:     srv.URL,
		Headers: map[string]string{"X-Test": "1", "Authorization": "Bearer t"},
		Body:    `{"a":1}`,
	})
	if err != nil {
		t.Fatalf("ExecuteAdHoc: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotHeader != "1" {
		t.Errorf("X-Test header = %q, want 1", gotHeader)
	}
	if !strings.Contains(gotBody, `"a":1`) {
		t.Errorf("body = %q, want it to contain the JSON", gotBody)
	}
}

func TestExecuteAdHocRejectsSeparatorBody(t *testing.T) {
	m := newTestManager(t)
	_, err := m.ExecuteAdHoc(context.Background(), AdHocReq{
		Method: "POST",
		URL:    "https://example.com",
		Body:   "line1\n### looks like a separator\nline3",
	})
	if err == nil {
		t.Fatal("a body containing a ### line should be rejected")
	}
}

func TestBuildAdHocContent(t *testing.T) {
	out := buildAdHocContent("POST", "https://x/y",
		map[string]string{"X-Test": "1", "Authorization": "Bearer t"}, `{"a":1}`)

	if !strings.Contains(out, "POST https://x/y\n") {
		t.Fatalf("missing request line:\n%s", out)
	}
	ai, xi := strings.Index(out, "Authorization:"), strings.Index(out, "X-Test:")
	if ai < 0 || xi < 0 || ai > xi {
		t.Fatalf("headers should be sorted (Authorization before X-Test):\n%s", out)
	}
	if !strings.Contains(out, "\n\n{\"a\":1}") {
		t.Fatalf("body should follow a blank line:\n%s", out)
	}

	// Whitespace-only body adds no body block.
	bare := buildAdHocContent("GET", "https://x", nil, "   ")
	if bare != "### Ad-hoc request\nGET https://x\n" {
		t.Fatalf("unexpected bare content: %q", bare)
	}
}
