// Package sse provides Server-Sent Events (SSE) client support for hitspec.
package sse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Event represents a single SSE event.
type Event struct {
	ID    string
	Type  string
	Data  string
	Retry int
}

// Client is an SSE client that connects to an SSE endpoint and receives events.
type Client struct {
	httpClient *http.Client
	url        string
	headers    map[string]string
	timeout    time.Duration
	lastEventID string
}

// Option is a functional option for configuring an SSE Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithHeaders sets custom headers for the SSE request.
func WithHeaders(headers map[string]string) Option {
	return func(c *Client) {
		c.headers = headers
	}
}

// WithTimeout sets the connection timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithLastEventID sets the Last-Event-ID header for reconnection.
func WithLastEventID(id string) Option {
	return func(c *Client) {
		c.lastEventID = id
	}
}

// NewClient creates a new SSE client.
func NewClient(url string, opts ...Option) *Client {
	c := &Client{
		url:     url,
		headers: make(map[string]string),
		timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: c.timeout,
		}
	}

	return c
}

// StreamResult contains the results of an SSE stream.
type StreamResult struct {
	Events   []Event
	Error    error
	Duration time.Duration
}

// Stream connects to the SSE endpoint and collects events until the context is cancelled
// or the specified event count is reached.
func (c *Client) Stream(ctx context.Context, maxEvents int) *StreamResult {
	start := time.Now()
	result := &StreamResult{
		Events: make([]Event, 0),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Set custom headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Set Last-Event-ID if specified
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("failed to connect: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		result.Error = fmt.Errorf("unexpected content type: %s (expected text/event-stream)", contentType)
		result.Duration = time.Since(start)
		return result
	}

	// Parse events
	events, err := c.parseEvents(resp.Body, maxEvents)
	if err != nil && err != io.EOF {
		result.Error = err
	}
	result.Events = events
	result.Duration = time.Since(start)

	return result
}

// parseEvents reads and parses SSE events from a reader.
func (c *Client) parseEvents(reader io.Reader, maxEvents int) ([]Event, error) {
	scanner := bufio.NewScanner(reader)
	var events []Event
	var currentEvent Event
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line signals end of event
		if line == "" {
			if len(dataLines) > 0 {
				currentEvent.Data = strings.Join(dataLines, "\n")
				events = append(events, currentEvent)

				if maxEvents > 0 && len(events) >= maxEvents {
					return events, nil
				}

				// Reset for next event
				currentEvent = Event{}
				dataLines = nil
			}
			continue
		}

		// Parse field
		if strings.HasPrefix(line, ":") {
			// Comment, ignore
			continue
		}

		var field, value string
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			field = line
			value = ""
		} else {
			field = line[:colonIdx]
			value = line[colonIdx+1:]
			// Remove leading space from value
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
		}

		switch field {
		case "event":
			currentEvent.Type = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			currentEvent.ID = value
		case "retry":
			// Parse retry interval (ignored for now)
		}
	}

	// Handle any remaining event
	if len(dataLines) > 0 {
		currentEvent.Data = strings.Join(dataLines, "\n")
		events = append(events, currentEvent)
	}

	return events, scanner.Err()
}

// EventHandler is a callback for handling SSE events.
type EventHandler func(event Event)

// StreamWithHandler connects to the SSE endpoint and calls the handler for each event.
func (c *Client) StreamWithHandler(ctx context.Context, handler EventHandler) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Set custom headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Set Last-Event-ID if specified
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		return fmt.Errorf("unexpected content type: %s", contentType)
	}

	scanner := bufio.NewScanner(resp.Body)
	var currentEvent Event
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Empty line signals end of event
		if line == "" {
			if len(dataLines) > 0 {
				currentEvent.Data = strings.Join(dataLines, "\n")
				handler(currentEvent)

				// Reset for next event
				currentEvent = Event{}
				dataLines = nil
			}
			continue
		}

		// Parse field
		if strings.HasPrefix(line, ":") {
			continue
		}

		var field, value string
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			field = line
			value = ""
		} else {
			field = line[:colonIdx]
			value = line[colonIdx+1:]
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
		}

		switch field {
		case "event":
			currentEvent.Type = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			currentEvent.ID = value
		}
	}

	return scanner.Err()
}
