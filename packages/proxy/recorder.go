// Package proxy provides an HTTP proxy that records requests and responses.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Recording represents a recorded HTTP request/response pair
type Recording struct {
	Timestamp   time.Time
	Method      string
	URL         string
	Path        string
	Headers     map[string]string
	Body        string
	ContentType string
	Response    *RecordedResponse
}

// RecordedResponse represents a recorded HTTP response
type RecordedResponse struct {
	StatusCode  int
	Status      string
	Headers     map[string]string
	Body        string
	ContentType string
	Duration    time.Duration
}

// Recorder is an HTTP proxy that records requests
type Recorder struct {
	port        int
	targetURL   string
	recordings  []Recording
	mutex       sync.Mutex
	verbose     bool
	exclude     []string
	sanitize    []string // Headers to redact
	deduplicate bool
	seen        map[string]bool
}

// Option is a functional option for Recorder
type Option func(*Recorder)

// WithPort sets the proxy port
func WithPort(port int) Option {
	return func(r *Recorder) {
		r.port = port
	}
}

// WithTargetURL sets the target URL to proxy to
func WithTargetURL(target string) Option {
	return func(r *Recorder) {
		r.targetURL = target
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(r *Recorder) {
		r.verbose = verbose
	}
}

// WithExclude sets paths to exclude from recording
func WithExclude(paths []string) Option {
	return func(r *Recorder) {
		r.exclude = paths
	}
}

// WithSanitize sets headers to redact
func WithSanitize(headers []string) Option {
	return func(r *Recorder) {
		r.sanitize = headers
	}
}

// WithDeduplicate enables request deduplication
func WithDeduplicate(enabled bool) Option {
	return func(r *Recorder) {
		r.deduplicate = enabled
	}
}

// NewRecorder creates a new recording proxy
func NewRecorder(opts ...Option) *Recorder {
	r := &Recorder{
		port:       8080,
		recordings: make([]Recording, 0),
		sanitize:   []string{"Authorization", "Cookie", "X-Api-Key", "Api-Key"},
		seen:       make(map[string]bool),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Start starts the recording proxy
func (r *Recorder) Start() error {
	if r.targetURL == "" {
		return fmt.Errorf("target URL is required")
	}

	target, err := url.Parse(r.targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			return r.recordResponse(resp)
		},
	}

	addr := fmt.Sprintf(":%d", r.port)
	server := &http.Server{
		Addr:    addr,
		Handler: r.wrap(proxy),
	}

	log.Printf("Recording proxy starting on http://localhost:%d", r.port)
	log.Printf("Proxying to: %s", r.targetURL)

	return server.ListenAndServe()
}

// StartWithContext starts the proxy with context for graceful shutdown
func (r *Recorder) StartWithContext(ctx context.Context) error {
	if r.targetURL == "" {
		return fmt.Errorf("target URL is required")
	}

	target, err := url.Parse(r.targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			return r.recordResponse(resp)
		},
	}

	addr := fmt.Sprintf(":%d", r.port)
	server := &http.Server{
		Addr:    addr,
		Handler: r.wrap(proxy),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("Recording proxy starting on http://localhost:%d", r.port)
	log.Printf("Proxying to: %s", r.targetURL)

	return server.ListenAndServe()
}

func (r *Recorder) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()

		// Check exclusion
		if r.shouldExclude(req.URL.Path) {
			if r.verbose {
				log.Printf("Excluded: %s %s", req.Method, req.URL.Path)
			}
			next.ServeHTTP(w, req)
			return
		}

		// Read and store body
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Create recording
		recording := Recording{
			Timestamp:   start,
			Method:      req.Method,
			URL:         req.URL.String(),
			Path:        req.URL.Path,
			Headers:     r.sanitizeHeaders(req.Header),
			Body:        string(bodyBytes),
			ContentType: req.Header.Get("Content-Type"),
		}

		// Store in context for response handling
		ctx := context.WithValue(req.Context(), "recording", &recording)
		req = req.WithContext(ctx)

		// Serve request
		next.ServeHTTP(w, req)

		// Recording is completed in recordResponse
	})
}

func (r *Recorder) recordResponse(resp *http.Response) error {
	// Get recording from context
	recording, ok := resp.Request.Context().Value("recording").(*Recording)
	if !ok {
		return nil
	}

	// Read response body
	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Create response recording
	recording.Response = &RecordedResponse{
		StatusCode:  resp.StatusCode,
		Status:      resp.Status,
		Headers:     r.sanitizeHeaders(resp.Header),
		Body:        string(bodyBytes),
		ContentType: resp.Header.Get("Content-Type"),
		Duration:    time.Since(recording.Timestamp),
	}

	// Check deduplication
	if r.deduplicate {
		key := recording.Method + ":" + recording.Path
		r.mutex.Lock()
		if r.seen[key] {
			r.mutex.Unlock()
			if r.verbose {
				log.Printf("Skipped duplicate: %s %s", recording.Method, recording.Path)
			}
			return nil
		}
		r.seen[key] = true
		r.mutex.Unlock()
	}

	// Store recording
	r.mutex.Lock()
	r.recordings = append(r.recordings, *recording)
	r.mutex.Unlock()

	if r.verbose {
		log.Printf("Recorded: %s %s -> %d (%s)", recording.Method, recording.Path, resp.StatusCode, recording.Response.Duration)
	}

	return nil
}

func (r *Recorder) shouldExclude(path string) bool {
	for _, exclude := range r.exclude {
		if strings.HasPrefix(path, exclude) {
			return true
		}
		if strings.Contains(path, exclude) {
			return true
		}
	}
	return false
}

func (r *Recorder) sanitizeHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range h {
		if len(values) > 0 {
			// Check if header should be redacted
			shouldRedact := false
			for _, s := range r.sanitize {
				if strings.EqualFold(key, s) {
					shouldRedact = true
					break
				}
			}

			if shouldRedact {
				result[key] = "{{" + strings.ToUpper(strings.ReplaceAll(key, "-", "_")) + "}}"
			} else {
				result[key] = values[0]
			}
		}
	}
	return result
}

// GetRecordings returns all recorded requests
func (r *Recorder) GetRecordings() []Recording {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	result := make([]Recording, len(r.recordings))
	copy(result, r.recordings)
	return result
}

// Clear clears all recordings
func (r *Recorder) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.recordings = make([]Recording, 0)
	r.seen = make(map[string]bool)
}

// Export exports recordings to hitspec format
func (r *Recorder) Export() string {
	recordings := r.GetRecordings()
	return ExportRecordings(recordings)
}

// ExportToJSON exports recordings to JSON format
func (r *Recorder) ExportToJSON() ([]byte, error) {
	recordings := r.GetRecordings()
	return json.MarshalIndent(recordings, "", "  ")
}
