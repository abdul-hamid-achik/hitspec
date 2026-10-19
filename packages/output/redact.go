package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// redactedValue replaces the value of a sensitive header or URL parameter.
const redactedValue = "[REDACTED]"

// sensitiveHeaders lists header names whose values are always redacted.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
}

// sensitiveFragments catches the rest of the "X-Api-Key-like" names
// (X-Custom-Api-Key, X-Amz-Security-Token, X-Client-Secret, ...). Names are
// compared lower-cased with "_" folded to "-".
var sensitiveFragments = []string{"api-key", "apikey", "token", "secret", "password"}

// isSensitiveName reports whether a header or query-parameter name carries a
// credential. Every formatter goes through it so they redact the same things.
func isSensitiveName(name string) bool {
	n := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "_", "-")
	if sensitiveHeaders[n] {
		return true
	}
	for _, frag := range sensitiveFragments {
		if strings.Contains(n, frag) {
			return true
		}
	}
	return false
}

// redactHeaders returns a copy of headers with sensitive values replaced.
func redactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitiveName(k) {
			redacted[k] = redactedValue
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// RedactURL masks the password of a URL's userinfo and the value of every
// query parameter whose name looks like a credential (api_key, access_token,
// ...). It edits the text in place, so everything else is returned untouched.
func RedactURL(raw string) string {
	if !strings.ContainsAny(raw, "@?") {
		return raw
	}
	rest, fragment := raw, ""
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest, fragment = rest[:i], rest[i:]
	}
	query := ""
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		rest, query = rest[:i], rest[i:]
	}

	// scheme://user:password@host/path
	if i := strings.Index(rest, "://"); i >= 0 {
		authStart := i + len("://")
		authEnd := len(rest)
		if j := strings.IndexByte(rest[authStart:], '/'); j >= 0 {
			authEnd = authStart + j
		}
		if at := strings.LastIndexByte(rest[authStart:authEnd], '@'); at >= 0 {
			userinfo := rest[authStart : authStart+at]
			if user, _, hasPassword := strings.Cut(userinfo, ":"); hasPassword {
				rest = rest[:authStart] + user + ":" + redactedValue + rest[authStart+at:]
			}
		}
	}

	if query != "" {
		pairs := strings.Split(query[1:], "&")
		for i, pair := range pairs {
			key, value, hasValue := strings.Cut(pair, "=")
			// An unresolved {{reference}} is a template, not a secret.
			if !hasValue || strings.Contains(value, "{{") || strings.Contains(value, "%7B%7B") {
				continue
			}
			name := key
			if decoded, err := url.QueryUnescape(key); err == nil {
				name = decoded
			}
			if isSensitiveName(name) {
				pairs[i] = key + "=" + redactedValue
			}
		}
		query = "?" + strings.Join(pairs, "&")
	}
	return rest + query + fragment
}

var (
	queryPairInText = regexp.MustCompile(`([?&])([^=&\s"'#]+)=([^&\s"'#]*)`)
	userinfoInText  = regexp.MustCompile(`(://[^/\s:@]*):[^@/\s]+@`)
)

// RedactText masks credentials inside free text that embeds a URL, such as
// the error Go reports for a failed request (`Get "http://h/x?api_key=...":
// dial tcp ...`): the password of a user:password@ and the value of query
// parameters whose name looks like a credential.
func RedactText(s string) string {
	if !strings.ContainsAny(s, "?@") {
		return s
	}
	s = userinfoInText.ReplaceAllString(s, "${1}:"+redactedValue+"@")
	return queryPairInText.ReplaceAllStringFunc(s, func(m string) string {
		parts := queryPairInText.FindStringSubmatch(m)
		name, value := parts[2], parts[3]
		if strings.Contains(value, "{{") || strings.Contains(value, "%7B%7B") {
			return m
		}
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
		if isSensitiveName(name) {
			return parts[1] + parts[2] + "=" + redactedValue
		}
		return m
	})
}

// sortedHeaderNames returns the keys of headers in a stable order.
func sortedHeaderNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Limits of the body excerpt shown for a failed request.
const (
	excerptMaxBytes = 2048
	excerptMaxLines = 30
)

// bodyText is a response or request body prepared for display.
type bodyText struct {
	Text      string // pretty-printed (JSON) and, unless full, capped
	Total     int    // size of the original body in bytes
	Truncated bool
	Note      string // "... 1234 more bytes, 56 more lines" when Truncated
	Binary    bool   // not text: Text is a one-line description instead
}

// formatBody prepares a body for display. JSON is pretty-printed. With
// limit set the result is capped at excerptMaxBytes / excerptMaxLines and Note
// says how much was cut; without it the whole body is kept. Binary bodies are
// described instead of dumped.
func formatBody(body []byte, limit bool) bodyText {
	out := bodyText{Total: len(body)}
	if len(body) == 0 {
		return out
	}
	if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		out.Binary = true
		out.Text = fmt.Sprintf("[%d bytes, binary]", len(body))
		return out
	}
	text := string(body)
	if json.Valid(body) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err == nil {
			text = buf.String()
		}
	}
	text = strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if !limit {
		out.Text = text
		return out
	}

	cut := len(text)
	lines := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines++
			if lines >= excerptMaxLines {
				cut = i
				break
			}
		}
	}
	if cut > excerptMaxBytes {
		cut = excerptMaxBytes
		// Do not split a multi-byte rune.
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
	}
	if cut >= len(text) {
		out.Text = text
		return out
	}
	rest := text[cut:]
	out.Text = text[:cut]
	out.Truncated = true
	moreLines := strings.Count(strings.TrimLeft(rest, "\n"), "\n") + 1
	out.Note = fmt.Sprintf("... %d more bytes, %d more lines", len(rest), moreLines)
	return out
}

// sanitize makes untrusted text (a response body, a header value) safe to
// print on a terminal: control characters other than tab and newline, which
// could carry escape sequences, become U+FFFD.
func sanitize(s string) string {
	clean := true
	for _, r := range s {
		if isUnsafeRune(r) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isUnsafeRune(r) {
			return '�'
		}
		return r
	}, s)
}

func isUnsafeRune(r rune) bool {
	return (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r < 0xa0)
}
