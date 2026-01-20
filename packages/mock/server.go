// Package mock provides a mock HTTP server that serves responses based on hitspec files.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/builtin"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// Server is a mock HTTP server based on hitspec files
type Server struct {
	router   *Router
	port     int
	delay    time.Duration
	verbose  bool
	registry *builtin.Registry
}

// Option is a functional option for Server
type Option func(*Server)

// WithPort sets the server port
func WithPort(port int) Option {
	return func(s *Server) {
		s.port = port
	}
}

// WithDelay adds a delay to all responses
func WithDelay(delay time.Duration) Option {
	return func(s *Server) {
		s.delay = delay
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(s *Server) {
		s.verbose = verbose
	}
}

// NewServer creates a new mock server
func NewServer(opts ...Option) *Server {
	s := &Server{
		router:   NewRouter(),
		port:     3000,
		registry: builtin.NewRegistry(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// LoadFile loads routes from a hitspec file
func (s *Server) LoadFile(path string) error {
	file, err := parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", path, err)
	}

	return s.LoadParsedFile(file)
}

// LoadParsedFile loads routes from a parsed file
func (s *Server) LoadParsedFile(file *parser.File) error {
	// Process file-level variables
	vars := make(map[string]string)
	for _, v := range file.Variables {
		vars[v.Name] = v.Value
	}

	for _, req := range file.Requests {
		route := s.createRoute(req, vars)
		if route != nil {
			s.router.AddRoute(route)
		}
	}

	return nil
}

// LoadFiles loads routes from multiple hitspec files
func (s *Server) LoadFiles(paths []string) error {
	for _, path := range paths {
		if err := s.LoadFile(path); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createRoute(req *parser.Request, vars map[string]string) *Route {
	// Resolve URL with variables
	url := s.resolveVariables(req.URL, vars)

	// Extract path pattern (strip query params and base URL)
	pathPattern := extractPathPattern(url)

	route := &Route{
		Method:      req.Method,
		PathPattern: pathPattern,
		PathRegex:   createPathRegex(pathPattern),
		Name:        req.Name,
		Response:    s.createMockResponse(req),
	}

	return route
}

func (s *Server) resolveVariables(input string, vars map[string]string) string {
	result := input

	// Replace {{variable}} patterns
	varPattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	result = varPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name (without braces)
		name := strings.TrimPrefix(strings.TrimSuffix(match, "}}"), "{{")
		name = strings.TrimSpace(name)

		// Check if it's a function call
		if strings.HasPrefix(name, "$") {
			funcExpr := strings.TrimPrefix(name, "$")
			if val, ok := s.registry.Call(funcExpr); ok {
				return fmt.Sprintf("%v", val)
			}
		}

		// Check variables
		if val, ok := vars[name]; ok {
			return val
		}

		// Check environment
		if val := os.Getenv(name); val != "" {
			return val
		}

		// Keep original if not found
		return match
	})

	return result
}

func extractPathPattern(url string) string {
	// Remove scheme and host
	if idx := strings.Index(url, "://"); idx != -1 {
		url = url[idx+3:]
		if idx := strings.Index(url, "/"); idx != -1 {
			url = url[idx:]
		} else {
			url = "/"
		}
	}

	// Remove query string
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// Ensure path starts with /
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}

	return url
}

func createPathRegex(pattern string) *regexp.Regexp {
	// Convert {{param}} to named capture groups
	regexPattern := regexp.MustCompile(`\{\{([^}]+)\}\}`).ReplaceAllString(pattern, `(?P<$1>[^/]+)`)

	// Escape other special chars but preserve capture groups
	// Simple approach: just compile as-is since we converted params
	regex, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		// Fallback to literal match
		return regexp.MustCompile("^" + regexp.QuoteMeta(pattern) + "$")
	}
	return regex
}

func (s *Server) createMockResponse(req *parser.Request) *MockResponse {
	resp := &MockResponse{
		StatusCode:  200,
		ContentType: "application/json",
		Headers:     make(map[string]string),
	}

	// Check for mock block in the request (>>>mock ... <<<)
	// For now, generate from assertions
	if len(req.Assertions) > 0 {
		resp.Body = s.generateBodyFromAssertions(req.Assertions)
	} else {
		// Default response
		resp.Body = `{"status": "ok", "message": "Mock response"}`
	}

	// Try to infer status from assertions
	for _, a := range req.Assertions {
		if a.Subject == "status" && a.Expected != nil {
			if status, ok := a.Expected.(int); ok {
				resp.StatusCode = status
			}
		}
	}

	return resp
}

func (s *Server) generateBodyFromAssertions(assertions []*parser.Assertion) string {
	// Build a JSON object from body assertions
	bodyData := make(map[string]interface{})

	for _, a := range assertions {
		if strings.HasPrefix(a.Subject, "body.") {
			path := strings.TrimPrefix(a.Subject, "body.")
			// Simple case: direct field
			if !strings.Contains(path, ".") && !strings.Contains(path, "[") {
				if a.Expected != nil {
					bodyData[path] = a.Expected
				}
			}
		} else if a.Subject == "body" {
			// Return the expected body directly
			if str, ok := a.Expected.(string); ok {
				return str
			}
			if data, err := json.Marshal(a.Expected); err == nil {
				return string(data)
			}
		}
	}

	if len(bodyData) == 0 {
		return `{"status": "ok"}`
	}

	data, err := json.MarshalIndent(bodyData, "", "  ")
	if err != nil {
		return `{"status": "ok"}`
	}
	return string(data)
}

// Start starts the mock server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Mock server starting on http://localhost:%d", s.port)
	log.Printf("Routes loaded: %d", len(s.router.routes))

	if s.verbose {
		for _, route := range s.router.routes {
			log.Printf("  %s %s -> %d", route.Method, route.PathPattern, route.Response.StatusCode)
		}
	}

	return server.ListenAndServe()
}

// StartWithContext starts the server with context for graceful shutdown
func (s *Server) StartWithContext(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("Mock server starting on http://localhost:%d", s.port)
	return server.ListenAndServe()
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Apply delay if configured
	if s.delay > 0 {
		time.Sleep(s.delay)
	}

	// Find matching route
	route, params := s.router.Match(r.Method, r.URL.Path)

	if route == nil {
		if s.verbose {
			log.Printf("%s %s -> 404 Not Found (%s)", r.Method, r.URL.Path, time.Since(start))
		}
		http.NotFound(w, r)
		return
	}

	// Build response
	resp := route.Response

	// Set headers
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}
	w.Header().Set("Content-Type", resp.ContentType)

	// Resolve body with params
	body := s.resolveBodyParams(resp.Body, params)

	w.WriteHeader(resp.StatusCode)
	w.Write([]byte(body))

	if s.verbose {
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, resp.StatusCode, time.Since(start))
	}
}

func (s *Server) resolveBodyParams(body string, params map[string]string) string {
	result := body
	for key, value := range params {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

// GetRoutes returns all registered routes
func (s *Server) GetRoutes() []*Route {
	return s.router.routes
}
