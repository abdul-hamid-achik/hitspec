package output

import (
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
)

// requestLine is "METHOD url" for the request as sent (URL credentials masked),
// or "" when the request never got built.
func requestLine(r *runner.RequestResult) string {
	if r.Request == nil {
		return ""
	}
	return sanitize(r.Request.Method) + " " + sanitize(RedactURL(r.Request.URL))
}

// responseStatus is "404 Not Found" (the status line), or "" without a response.
func responseStatus(r *runner.RequestResult) string {
	if r.Response == nil {
		return ""
	}
	if s := sanitize(r.Response.Status); s != "" {
		return s
	}
	return fmt.Sprintf("%d", r.Response.StatusCode)
}

// responseMillis is how long the response took, falling back to the request's.
func responseMillis(r *runner.RequestResult) int64 {
	if r.Response != nil && r.Response.Duration > 0 {
		return r.Response.Duration.Milliseconds()
	}
	return r.Duration.Milliseconds()
}

// failureContext renders the request line, the response status line and a
// capped body excerpt of a failed request as plain text (for JUnit). It is ""
// when there is nothing to show.
func failureContext(r *runner.RequestResult) string {
	var sb strings.Builder
	if line := requestLine(r); line != "" {
		fmt.Fprintf(&sb, "Request: %s\n", line)
	}
	if status := responseStatus(r); status != "" {
		fmt.Fprintf(&sb, "Response: %s (%dms)\n", status, responseMillis(r))
		b := formatBody(r.Response.Body, true)
		if b.Total == 0 {
			sb.WriteString("Body: (empty)\n")
		} else {
			sb.WriteString("Body:\n")
			sb.WriteString(sanitize(b.Text))
			sb.WriteString("\n")
			if b.Truncated {
				sb.WriteString(b.Note + "\n")
			}
		}
	}
	return sb.String()
}
