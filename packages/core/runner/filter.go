package runner

import (
	"fmt"
	"os"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// shouldRun determines whether a request should be executed based on @only,
// name filter, and tag filter configuration.
func (r *Runner) shouldRun(req *parser.Request, hasOnly bool) bool {
	if hasOnly && (req.Metadata == nil || !req.Metadata.Only) {
		return false
	}

	if r.config.NameFilter != "" {
		if req.Name == "" || !matchesPattern(req.Name, r.config.NameFilter) {
			return false
		}
	}

	if len(r.config.TagsFilter) > 0 {
		if !parser.HasAnyTag(req.Tags, r.config.TagsFilter) {
			return false
		}
	}

	return true
}

// matchesPattern checks if a name matches a glob-like pattern with optional
// leading/trailing wildcards (*).
func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}

	if pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		substr := pattern[1 : len(pattern)-1]
		for i := 0; i <= len(name)-len(substr); i++ {
			if name[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}

	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix
	}

	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}

	return name == pattern
}

// topologicalSort returns requests in dependency-respecting order using Kahn's algorithm.
func (r *Runner) topologicalSort(requests []*parser.Request) ([]*parser.Request, error) {
	// Build adjacency list and in-degree count
	inDegree := make(map[string]int)
	adjacency := make(map[string][]string)
	requestMap := make(map[string]*parser.Request)

	// Initialize all requests
	for _, req := range requests {
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("__anon_%p", req)
		}
		inDegree[name] = 0
		requestMap[name] = req
	}

	// Build graph from dependencies
	for _, req := range requests {
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("__anon_%p", req)
		}

		if req.Metadata != nil && len(req.Metadata.Depends) > 0 {
			for _, dep := range req.Metadata.Depends {
				if _, exists := requestMap[dep]; exists {
					adjacency[dep] = append(adjacency[dep], name)
					inDegree[name]++
				} else {
					fmt.Fprintf(os.Stderr, "warning: request %q depends on %q which does not exist\n", name, dep)
				}
			}
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var sortedNames []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		sortedNames = append(sortedNames, current)

		for _, neighbor := range adjacency[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Check for cycles
	if len(sortedNames) != len(requests) {
		// Find which requests are part of the cycle (have remaining in-degree > 0)
		var cycleMembers []string
		for name, degree := range inDegree {
			if degree > 0 {
				cycleMembers = append(cycleMembers, name)
			}
		}
		return nil, fmt.Errorf("circular dependency detected involving requests: %v", cycleMembers)
	}

	// Map sorted names back to requests, preserving original order for requests
	// with same dependency level
	var sorted []*parser.Request
	processedNames := make(map[string]bool)

	// First, add sorted requests in dependency order
	for _, name := range sortedNames {
		if req, exists := requestMap[name]; exists && !processedNames[name] {
			sorted = append(sorted, req)
			processedNames[name] = true
		}
	}

	return sorted, nil
}
