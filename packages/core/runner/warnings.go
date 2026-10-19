package runner

import (
	"fmt"
	neturl "net/url"
	"sort"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
)

// unresolvedWarnings reports every {{reference}} still present in the request
// that is about to be sent: the resolver leaves a reference it cannot resolve
// as literal text, so the server receives "{{login.token}}" verbatim. It scans
// the built request rather than re-resolving, which would re-run functions
// such as $uuid().
func unresolvedWarnings(req *http.Request) []string {
	if req == nil {
		return nil
	}
	var warnings []string
	seen := map[string]bool{}
	scan := func(where, text string) {
		for _, ref := range unresolvedRef.FindAllString(text, -1) {
			key := where + "\x00" + ref
			if seen[key] {
				continue
			}
			seen[key] = true
			warnings = append(warnings, fmt.Sprintf("%s was not resolved and was sent literally (%s)", ref, where))
		}
	}

	// Query values are percent-encoded when the URL is built, which hides the
	// braces; decode before looking.
	sentURL := req.URL
	if decoded, err := neturl.QueryUnescape(sentURL); err == nil {
		sentURL = decoded
	}
	scan("in the URL", sentURL)
	names := make([]string, 0, len(req.Headers))
	for name := range req.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		scan("in header "+name, req.Headers[name])
	}
	scan("in the body", req.Body)
	for _, field := range req.Multipart {
		if field != nil {
			scan("in multipart field "+field.Name, field.Value)
		}
	}
	return warnings
}
