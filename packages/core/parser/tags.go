package parser

import "strings"

// HasAnyTag returns true if any of the tags match any of the filters.
// Comparison is case-insensitive and tags are trimmed of whitespace.
func HasAnyTag(tags, filters []string) bool {
	for _, filter := range filters {
		f := strings.TrimSpace(strings.ToLower(filter))
		for _, tag := range tags {
			if strings.ToLower(strings.TrimSpace(tag)) == f {
				return true
			}
		}
	}
	return false
}
