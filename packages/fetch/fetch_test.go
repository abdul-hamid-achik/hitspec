package fetch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchPreservesBodyFollowsRedirectAndEnforcesLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final?secret=yes", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0x00, 0xff, 0x7f})
	}))
	defer server.Close()
	result, err := NewService().Fetch(context.Background(), Request{URL: server.URL + "/start", FollowRedirects: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Body) != string([]byte{0x00, 0xff, 0x7f}) || result.FinalURL != server.URL+"/final?secret=yes" {
		t.Fatalf("unexpected result: %#v", result)
	}
	tooLarge, err := NewService().Fetch(context.Background(), Request{URL: server.URL + "/final", MaxBodyBytes: 2})
	if tooLarge != nil || !IsBodyTooLarge(err) {
		t.Fatalf("result=%#v err=%v, want size error", tooLarge, err)
	}
}

func TestFetchAddsUserAgentWhenHeadersAreNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "hitspec-web-capture" {
			t.Errorf("User-Agent = %q", got)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	result, err := NewService().Fetch(context.Background(), Request{
		URL: server.URL, UserAgent: "hitspec-web-capture",
	})
	if err != nil || string(result.Body) != "ok" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestFetchHonorsCancellationAndPublicPolicy(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := NewService().Fetch(ctx, Request{URL: server.URL, Timeout: time.Minute})
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fetch did not stop")
	}
	_, err := NewService().Fetch(context.Background(), Request{URL: "http://127.0.0.1/", NetworkPolicy: NetworkPublicOnly})
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("error = %v, want non-public rejection", err)
	}
}

func TestRenderFormatsAreByteSafeReadableAndSanitized(t *testing.T) {
	raw := []byte{0, 1, 0xff}
	got, err := Render(context.Background(), &Result{Body: raw}, FormatRaw)
	if err != nil || string(got) != string(raw) {
		t.Fatalf("raw=%v err=%v", got, err)
	}
	htmlResult := &Result{
		RequestedURL: "https://user:pass@example.com/docs/page?token=secret", FinalURL: "https://example.com/docs/page?session=secret",
		Status: "200 OK", StatusCode: 200, ContentType: "text/html; charset=utf-8", Duration: time.Millisecond,
		Body: []byte(`<html><head><title>Hidden</title></head><body><h1>Hello</h1><a href="next">Next</a><script>bad()</script></body></html>`),
	}
	text, err := Render(context.Background(), htmlResult, FormatText)
	if err != nil || !strings.Contains(string(text), "Hello") || strings.Contains(string(text), "Hidden") || strings.Contains(string(text), "bad") {
		t.Fatalf("text=%q err=%v", text, err)
	}
	markdown, err := Render(context.Background(), htmlResult, FormatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"user:pass", "token=", "session="} {
		if strings.Contains(string(markdown), forbidden) {
			t.Fatalf("Markdown leaked %q: %s", forbidden, markdown)
		}
	}
	if !strings.Contains(string(markdown), "[Next](https://example.com/docs/next)") {
		t.Fatalf("relative link was not resolved: %s", markdown)
	}
}

func TestRenderJSONUsesBase64AndRedactsHeaders(t *testing.T) {
	body := []byte{0xff, 0, 1}
	result := &Result{
		RequestedURL: "https://example.com/file?token=secret", FinalURL: "https://example.com/file?token=secret",
		Status: "200 OK", StatusCode: 200, ContentType: "application/octet-stream", Body: body,
		Headers: http.Header{"Set-Cookie": {"secret=yes"}, "X-Trace": {"safe"}},
	}
	encoded, err := Render(context.Background(), result, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["body_encoding"] != "base64" || decoded["body"] != base64.StdEncoding.EncodeToString(body) {
		t.Fatalf("unexpected JSON: %s", encoded)
	}
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "Set-Cookie") || !strings.Contains(string(encoded), "X-Trace") {
		t.Fatalf("redaction failed: %s", encoded)
	}
}

func TestFetchRejectsOversizeStreamingResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Trailer", "X-End")
		_, _ = io.WriteString(w, strings.Repeat("x", 65))
	}))
	defer server.Close()
	_, err := NewService().Fetch(context.Background(), Request{URL: server.URL, MaxBodyBytes: 64})
	if !IsBodyTooLarge(err) {
		t.Fatalf("error = %v, want size error", err)
	}
}

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return r[host], nil
}

func TestPublicPolicyRejectsMixedAndNAT64Answers(t *testing.T) {
	resolver := staticResolver{
		"mixed.example": {{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("169.254.169.254")}},
		"nat64.example": {{IP: net.ParseIP("64:ff9b::7f00:1")}},
	}
	for _, host := range []string{"mixed.example", "nat64.example"} {
		if _, err := resolvePublic(context.Background(), resolver, host); err == nil || !strings.Contains(err.Error(), "non-public") {
			t.Fatalf("host %s error = %v, want non-public rejection", host, err)
		}
	}
}
