package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Stream_BasicEvents(t *testing.T) {
	// Create test server that sends SSE events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected response writer to support flushing")
		}

		// Send events
		_, _ = w.Write([]byte("event: message\ndata: hello\n\n"))
		flusher.Flush()

		_, _ = w.Write([]byte("event: message\ndata: world\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := client.Stream(ctx, 2)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}

	if result.Events[0].Type != "message" || result.Events[0].Data != "hello" {
		t.Errorf("unexpected first event: %+v", result.Events[0])
	}

	if result.Events[1].Type != "message" || result.Events[1].Data != "world" {
		t.Errorf("unexpected second event: %+v", result.Events[1])
	}
}

func TestClient_Stream_MultiLineData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: line1\ndata: line2\ndata: line3\n\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := client.Stream(ctx, 1)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}

	expectedData := "line1\nline2\nline3"
	if result.Events[0].Data != expectedData {
		t.Errorf("expected data %q, got %q", expectedData, result.Events[0].Data)
	}
}

func TestClient_Stream_EventWithID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("id: 123\nevent: update\ndata: test\n\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := client.Stream(ctx, 1)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}

	if result.Events[0].ID != "123" {
		t.Errorf("expected ID 123, got %q", result.Events[0].ID)
	}
}

func TestClient_Stream_WrongContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := client.Stream(ctx, 1)

	if result.Error == nil {
		t.Fatal("expected error for wrong content type")
	}

	if !strings.Contains(result.Error.Error(), "unexpected content type") {
		t.Errorf("unexpected error: %v", result.Error)
	}
}

func TestClient_Stream_WithCustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom header
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithHeaders(map[string]string{
		"Authorization": "Bearer token123",
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := client.Stream(ctx, 1)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestClient_Stream_WithLastEventID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Last-Event-ID header
		if r.Header.Get("Last-Event-ID") != "42" {
			t.Errorf("expected Last-Event-ID header 42, got %q", r.Header.Get("Last-Event-ID"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithLastEventID("42"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := client.Stream(ctx, 1)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestClient_StreamWithHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: event1\n\ndata: event2\n\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var events []Event
	handler := func(event Event) {
		events = append(events, event)
		if len(events) >= 2 {
			cancel()
		}
	}

	err := client.StreamWithHandler(ctx, handler)
	// Context cancellation is expected
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestParseEvents_Comments(t *testing.T) {
	client := NewClient("")
	reader := strings.NewReader(": this is a comment\ndata: actual data\n\n")

	events, err := client.parseEvents(reader, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Data != "actual data" {
		t.Errorf("expected data 'actual data', got %q", events[0].Data)
	}
}

func TestClient_ParseBody(t *testing.T) {
	client := NewClient("")
	body := []byte("event: message\ndata: hello\n\nevent: update\ndata: world\n\n")

	events, err := client.ParseBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != "message" || events[0].Data != "hello" {
		t.Errorf("unexpected first event: %+v", events[0])
	}

	if events[1].Type != "update" || events[1].Data != "world" {
		t.Errorf("unexpected second event: %+v", events[1])
	}
}

func TestClient_ParseBody_Empty(t *testing.T) {
	client := NewClient("")
	events, err := client.ParseBody([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestClient_ParseBody_WithID(t *testing.T) {
	client := NewClient("")
	body := []byte("id: 99\nevent: tick\ndata: {\"count\":1}\n\n")

	events, err := client.ParseBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].ID != "99" {
		t.Errorf("expected ID 99, got %q", events[0].ID)
	}
	if events[0].Type != "tick" {
		t.Errorf("expected type 'tick', got %q", events[0].Type)
	}
	if events[0].Data != `{"count":1}` {
		t.Errorf("expected JSON data, got %q", events[0].Data)
	}
}

// TestClient_ParseBody_LargeLine guards the regression where bufio.Scanner's
// default 64KB token limit silently dropped a single SSE data line longer
// than 64KB. The scanner is now buffered to 1MB.
func TestClient_ParseBody_LargeLine(t *testing.T) {
	client := NewClient("")
	// 200KB single data line — well above the old 64KB limit.
	big := strings.Repeat("x", 200*1024)
	body := []byte("event: bulk\ndata: " + big + "\n\n")

	events, err := client.ParseBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if len(events[0].Data) != len(big) {
		t.Fatalf("data length = %d, want %d (line was truncated by the 64KB scanner limit)", len(events[0].Data), len(big))
	}
}
