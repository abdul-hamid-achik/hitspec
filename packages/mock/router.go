package mock

import (
	"regexp"
	"strings"
)

// Route represents a mock route
type Route struct {
	Method      string
	PathPattern string
	PathRegex   *regexp.Regexp
	Name        string
	Response    *MockResponse
}

// MockResponse represents a mock HTTP response
type MockResponse struct {
	StatusCode  int
	ContentType string
	Headers     map[string]string
	Body        string
}

// Router matches incoming requests to routes
type Router struct {
	routes []*Route
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes: make([]*Route, 0),
	}
}

// AddRoute adds a route to the router
func (r *Router) AddRoute(route *Route) {
	r.routes = append(r.routes, route)
}

// Match finds a route matching the given method and path
func (r *Router) Match(method, path string) (*Route, map[string]string) {
	// Normalize path
	path = normalizePath(path)

	for _, route := range r.routes {
		if !strings.EqualFold(route.Method, method) {
			continue
		}

		if params := matchPath(route, path); params != nil {
			return route, params
		}
	}

	return nil, nil
}

func normalizePath(path string) string {
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Remove trailing slash (except for root)
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}

func matchPath(route *Route, path string) map[string]string {
	// Try regex match first
	if route.PathRegex != nil {
		matches := route.PathRegex.FindStringSubmatch(path)
		if matches != nil {
			params := make(map[string]string)
			names := route.PathRegex.SubexpNames()
			for i, name := range names {
				if i > 0 && name != "" && i < len(matches) {
					params[name] = matches[i]
				}
			}
			return params
		}
	}

	// Exact match
	if route.PathPattern == path {
		return make(map[string]string)
	}

	return nil
}
