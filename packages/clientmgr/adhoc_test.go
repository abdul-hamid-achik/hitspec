package clientmgr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteAdHoc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	m := newTestManager(t)
	dto, err := m.ExecuteAdHoc(context.Background(), AdHocReq{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("ExecuteAdHoc: %v", err)
	}
	if dto.File != "(ad-hoc)" {
		t.Fatalf("file label = %q, want (ad-hoc)", dto.File)
	}
	if len(dto.Results) != 1 || dto.Results[0].Response == nil {
		t.Fatalf("expected one result with a response: %+v", dto.Results)
	}
	resp := dto.Results[0].Response
	if resp.StatusCode != 200 || !strings.Contains(resp.Body, "ok") {
		t.Fatalf("unexpected response: %d %q", resp.StatusCode, resp.Body)
	}
}

func TestExecuteAdHocRequiresURL(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.ExecuteAdHoc(context.Background(), AdHocReq{Method: "GET"}); err == nil {
		t.Fatal("ExecuteAdHoc without a URL should error")
	}
}

func TestExecuteAdHocDefaultsToGET(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := newTestManager(t)
	if _, err := m.ExecuteAdHoc(context.Background(), AdHocReq{URL: srv.URL}); err != nil {
		t.Fatalf("ExecuteAdHoc: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("empty method should default to GET, server saw %q", gotMethod)
	}
}
