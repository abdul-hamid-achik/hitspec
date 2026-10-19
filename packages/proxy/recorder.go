// Package proxy provides an HTTP proxy that records requests and responses.
package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/portprobe"
)

// DefaultHost is the address the proxy binds to unless WithHost says otherwise.
// The proxy forwards to an arbitrary target, so it must not be reachable from
// the network by default; use hitspec record --host 0.0.0.0 to expose it.
const DefaultHost = "127.0.0.1"

// DefaultMaxBodyBytes caps how much of each request/response body a recording
// keeps. The proxy still forwards the whole body; only the copy kept for the
// export is cut off.
const DefaultMaxBodyBytes = 1 << 20

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const recordingKey contextKey = "recording"

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
	// Truncated is true when the body was longer than the recorder's cap and
	// Body holds only its beginning.
	Truncated bool
}

// Recorder is an HTTP proxy that records requests
type Recorder struct {
	host        string
	port        int
	maxBody     int
	redact      bool
	targetURL   string
	recordings  []Recording
	mutex       sync.Mutex
	verbose     bool
	exclude     []string
	sanitize    []string // Headers to redact
	deduplicate bool
	seen        map[string]bool

	lmu      sync.Mutex
	listener net.Listener
}

// Option is a functional option for Recorder
type Option func(*Recorder)

// WithPort sets the proxy port
func WithPort(port int) Option {
	return func(r *Recorder) {
		r.port = port
	}
}

// WithHost sets the address to bind to (default 127.0.0.1).
func WithHost(host string) Option {
	return func(r *Recorder) {
		r.host = host
	}
}

// WithRedact turns redaction of recorded secrets on (the default) or off. When
// on, the default sensitive headers (Authorization, Cookie, Set-Cookie,
// X-Api-Key, ...) become {{PLACEHOLDER}} references and JSON fields named
// password, secret, token or apiKey (and their *_token / *Secret variants) are
// replaced the same way. Off records everything verbatim.
func WithRedact(enabled bool) Option {
	return func(r *Recorder) {
		r.redact = enabled
	}
}

// WithMaxBodyBytes caps the body kept per recording (default 1 MiB). Bodies are
// always forwarded in full.
func WithMaxBodyBytes(n int) Option {
	return func(r *Recorder) {
		r.maxBody = n
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
		host:       DefaultHost,
		port:       8080,
		maxBody:    DefaultMaxBodyBytes,
		redact:     true,
		recordings: make([]Recording, 0),
		sanitize:   []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-Api-Key", "Api-Key"},
		seen:       make(map[string]bool),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Listen binds the configured host and port. A busy port fails here, so callers
// can report it instead of announcing a proxy that is not running. Port 0 picks
// a free port; Addr returns it.
func (r *Recorder) Listen() (net.Addr, error) {
	r.lmu.Lock()
	defer r.lmu.Unlock()
	if r.listener != nil {
		return r.listener.Addr(), nil
	}
	hostPort := net.JoinHostPort(r.host, strconv.Itoa(r.port))
	// bind() can succeed next to a wildcard listener (macOS, SO_REUSEADDR):
	// probe first so that case is an error too.
	if portprobe.ShouldProbe(r.host) && portprobe.InUse(r.port) {
		return nil, fmt.Errorf("recording proxy cannot listen on %s: port %d is already in use", hostPort, r.port)
	}
	ln, err := net.Listen("tcp", hostPort)
	if err != nil {
		return nil, fmt.Errorf("recording proxy cannot listen on %s: %w", hostPort, err)
	}
	r.listener = ln
	return ln.Addr(), nil
}

// Addr returns the bound address, or nil before Listen/Start has bound it.
func (r *Recorder) Addr() net.Addr {
	r.lmu.Lock()
	defer r.lmu.Unlock()
	if r.listener == nil {
		return nil
	}
	return r.listener.Addr()
}

// Start starts the recording proxy and blocks until it fails.
func (r *Recorder) Start() error {
	return r.StartWithContext(context.Background())
}

// newReverseProxy forwards to target, joining the target's path prefix with
// the incoming path so upstream sees the same URL that fullURL records.
// Responses are streamed, not buffered: the reverse proxy flushes SSE and
// chunked (unknown length) bodies as they arrive while the recorder keeps a
// capped copy of what passes through.
func (r *Recorder) newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		ModifyResponse: func(resp *http.Response) error {
			return r.recordResponse(resp)
		},
	}
}

// StartWithContext starts the proxy with context for graceful shutdown. A busy
// or unusable port is returned as an error before anything is proxied.
func (r *Recorder) StartWithContext(ctx context.Context) error {
	if r.targetURL == "" {
		return fmt.Errorf("target URL is required")
	}

	target, err := url.Parse(r.targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	addr, err := r.Listen()
	if err != nil {
		return err
	}

	proxy := r.newReverseProxy(target)
	server := &http.Server{
		Handler:           r.wrap(proxy),
		ReadHeaderTimeout: 10 * time.Second,
	}

	done := make(chan struct{})
	defer close(done)
	defer func() {
		r.lmu.Lock()
		r.listener = nil
		r.lmu.Unlock()
	}()
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-done:
		}
	}()

	log.Printf("Recording proxy listening on http://%s", displayAddr(addr))
	log.Printf("Proxying to: %s", r.targetURL)

	r.lmu.Lock()
	ln := r.listener
	r.lmu.Unlock()
	if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// displayAddr renders a bound address for humans: an unspecified host shows as
// localhost.
func displayAddr(a net.Addr) string {
	host, port, err := net.SplitHostPort(a.String())
	if err != nil {
		return a.String()
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "localhost"
	}
	return net.JoinHostPort(host, port)
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

		// Create recording. The reverse proxy receives only the request path (no
		// scheme/host) in req.URL, so reconstruct the full runnable URL from the
		// configured target. Without this, recorded files emitted path-only URLs
		// that hitspec's own ValidateURL rejects, making recordings unrunnable.
		recording := Recording{
			Timestamp:   start,
			Method:      req.Method,
			URL:         r.fullURL(req.URL),
			Path:        req.URL.Path,
			Headers:     r.sanitizeHeaders(req.Header),
			Body:        r.redactBody(string(bodyBytes), req.Header.Get("Content-Type")),
			ContentType: req.Header.Get("Content-Type"),
		}

		// Store in context for response handling
		ctx := context.WithValue(req.Context(), recordingKey, &recording)
		req = req.WithContext(ctx)

		// Serve request
		next.ServeHTTP(w, req)

		// Recording is completed in recordResponse
	})
}

// recordResponse swaps the response body for one that tees what flows through
// to the client into a capped buffer and finishes the recording when the body
// ends (EOF) or the client goes away (Close). Nothing is buffered up front, so
// SSE and chunked responses are delivered incrementally.
func (r *Recorder) recordResponse(resp *http.Response) error {
	recording, ok := resp.Request.Context().Value(recordingKey).(*Recording)
	if !ok {
		return nil
	}
	body := resp.Body
	if body == nil {
		body = http.NoBody
	}
	rb := &recordingBody{
		src:     body,
		rec:     r,
		recd:    recording,
		resp:    resp,
		headers: r.sanitizeHeaders(resp.Header),
		max:     r.maxBody,
	}
	resp.Body = rb
	// Nothing will flow for these: record now, like a client expects.
	if body == http.NoBody || resp.ContentLength == 0 || resp.Request.Method == http.MethodHead {
		rb.finish()
	}
	return nil
}

// recordingBody is the tee described on recordResponse.
type recordingBody struct {
	src     io.ReadCloser
	rec     *Recorder
	recd    *Recording
	resp    *http.Response
	headers map[string]string
	max     int

	buf       bytes.Buffer
	total     int64
	truncated bool
	once      sync.Once
}

func (b *recordingBody) Read(p []byte) (int, error) {
	n, err := b.src.Read(p)
	if n > 0 {
		b.total += int64(n)
		b.capture(p[:n])
	}
	// Finish as soon as the declared length has been read, before the last
	// bytes reach the client: a client that stops at Content-Length never waits
	// on, or races with, the EOF read.
	if err != nil || (b.resp.ContentLength > 0 && b.total >= b.resp.ContentLength) {
		b.finish()
	}
	return n, err
}

func (b *recordingBody) Close() error {
	err := b.src.Close()
	b.finish()
	return err
}

func (b *recordingBody) capture(p []byte) {
	if b.max < 0 {
		return
	}
	room := b.max - b.buf.Len()
	if room <= 0 {
		b.truncated = true
		return
	}
	if len(p) > room {
		b.truncated = true
		p = p[:room]
	}
	b.buf.Write(p)
}

// finish stores the recording once. It runs on the goroutine that serves the
// response (Read hits EOF before the proxy handler returns), so a recording
// exists by the time the client has the whole body.
func (b *recordingBody) finish() {
	b.once.Do(func() {
		r, resp, recording := b.rec, b.resp, b.recd
		recordedBody := b.buf.Bytes()
		respHeaders := b.headers

		// Record a readable body: when the upstream response is gzip-encoded,
		// the raw bytes are binary garbage in the exported .http file. Decode
		// the gzip stream so the stored body is plain text, and drop the
		// Content-Encoding header so the recorded response is self-consistent.
		// The original encoded bytes were already forwarded to the client. A
		// truncated stream decodes as far as it goes.
		if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
			if decoded, err := decodeGzip(recordedBody); err == nil || (b.truncated && len(decoded) > 0) {
				recordedBody = decoded
				delete(respHeaders, "Content-Encoding")
			}
		}

		contentType := resp.Header.Get("Content-Type")
		out := &RecordedResponse{
			StatusCode:  resp.StatusCode,
			Status:      resp.Status,
			Headers:     respHeaders,
			Body:        r.redactBody(string(recordedBody), contentType),
			ContentType: contentType,
			Duration:    time.Since(recording.Timestamp),
			Truncated:   b.truncated,
		}

		// Work on a copy: the request-side Recording was built before the
		// response existed and is shared through the context.
		final := *recording
		final.Response = out
		r.store(&final)
	})
}

// store applies deduplication and appends the finished recording.
func (r *Recorder) store(recording *Recording) {
	r.mutex.Lock()
	if r.deduplicate {
		key := recording.Method + ":" + recording.Path
		if r.seen[key] {
			r.mutex.Unlock()
			if r.verbose {
				log.Printf("Skipped duplicate: %s %s", recording.Method, recording.Path)
			}
			return
		}
		r.seen[key] = true
	}
	r.recordings = append(r.recordings, *recording)
	r.mutex.Unlock()

	if r.verbose {
		log.Printf("Recorded: %s %s -> %d (%s)", recording.Method, recording.Path, recording.Response.StatusCode, recording.Response.Duration)
	}
}

// decodeGzip decompresses a gzip-encoded byte slice. It returns an error if the
// input is not a valid gzip stream so the caller can fall back to the raw bytes.
func decodeGzip(in []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return out, err
	}
	return out, nil
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

// sanitizeList is the header names to redact: none when redaction is off.
func (r *Recorder) sanitizeList() []string {
	if !r.redact {
		return nil
	}
	return r.sanitize
}

func (r *Recorder) sanitizeHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range h {
		if len(values) > 0 {
			// Check if header should be redacted
			shouldRedact := false
			for _, s := range r.sanitizeList() {
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

// fullURL reconstructs the full, runnable request URL the way the reverse proxy
// forwards it: the target's scheme, host, path prefix and query joined with the
// request's path and query. Falls back to the raw request URI when no (valid)
// target is configured.
func (r *Recorder) fullURL(reqURL *url.URL) string {
	if r.targetURL == "" {
		return reqURL.String()
	}
	target, err := url.Parse(r.targetURL)
	if err != nil || target.Host == "" {
		return strings.TrimRight(r.targetURL, "/") + reqURL.RequestURI()
	}
	out := *target
	out.Fragment, out.RawFragment = "", ""
	raw := joinURLPath(target.EscapedPath(), reqURL.EscapedPath())
	if p, err := url.PathUnescape(raw); err == nil {
		out.Path, out.RawPath = p, raw
	} else {
		out.Path, out.RawPath = raw, ""
	}
	switch {
	case target.RawQuery == "" || reqURL.RawQuery == "":
		out.RawQuery = target.RawQuery + reqURL.RawQuery
	default:
		out.RawQuery = target.RawQuery + "&" + reqURL.RawQuery
	}
	return out.String()
}

// joinURLPath joins two escaped URL paths with exactly one slash, like
// httputil.ReverseProxy does for the target's path prefix.
func joinURLPath(a, b string) string {
	aSlash, bSlash := strings.HasSuffix(a, "/"), strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash && a != "" && b != "":
		return a + "/" + b
	}
	if a+b == "" {
		return "/"
	}
	return a + b
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
