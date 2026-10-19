package mock

import (
	"regexp"
	"sort"
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

// AllowedMethods lists, sorted, the methods that have a route for path (HEAD is
// included when GET is, since the server answers HEAD from GET routes).
func (r *Router) AllowedMethods(path string) []string {
	path = normalizePath(path)
	seen := map[string]bool{}
	for _, route := range r.routes {
		if matchPath(route, path) != nil {
			seen[strings.ToUpper(route.Method)] = true
		}
	}
	if seen["GET"] {
		seen["HEAD"] = true
	}
	if len(seen) == 0 {
		return nil
	}
	methods := make([]string, 0, len(seen))
	for m := range seen {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	return methods
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
