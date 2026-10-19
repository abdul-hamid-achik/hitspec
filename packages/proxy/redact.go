package proxy

import (
	"encoding/json"
	"strings"
)

// redactBody hides secret-looking JSON fields in a request/response body when
// redaction is on and the body is JSON by content type or by shape. Other
// bodies are returned untouched.
func (r *Recorder) redactBody(body, contentType string) string {
	if !r.redact || body == "" {
		return body
	}
	if !strings.Contains(strings.ToLower(contentType), "json") {
		t := strings.TrimSpace(body)
		if t == "" || (t[0] != '{' && t[0] != '[') {
			return body
		}
	}
	return redactJSON(body)
}

// sensitiveKeySuffixes are matched against a JSON key lower-cased with "_" and
// "-" removed, so password, passwd, userPassword, access_token, clientSecret,
// api-key and apiKey are all caught but tokenType or secretary are not.
var sensitiveKeySuffixes = []string{"password", "passwd", "secret", "token", "apikey"}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	for _, suffix := range sensitiveKeySuffixes {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return false
}

// redactPlaceholder turns a key into the {{VARIABLE}} reference that replaces
// its value, e.g. "access_token" -> "{{ACCESS_TOKEN}}", "apiKey" -> "{{API_KEY}}".
func redactPlaceholder(key string) string {
	var b strings.Builder
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		case r >= 'A' && r <= 'Z':
			if i > 0 && key[i-1] >= 'a' && key[i-1] <= 'z' {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
		}
	}
	return "{{" + strings.Trim(b.String(), "_") + "}}"
}

// redactJSON replaces the value of every sensitive key with a placeholder
// string while leaving the rest of the text (key order, whitespace) alone. It
// scans rather than parses, so a body cut off by the recording cap is still
// redacted as far as it goes.
func redactJSON(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '"' {
			out.WriteByte(s[i])
			i++
			continue
		}
		end := stringEnd(s, i)
		out.WriteString(s[i:end])
		// A string followed by ':' is a key.
		j := skipSpace(s, end)
		if end > len(s) || j >= len(s) || s[j] != ':' {
			i = end
			continue
		}
		var key string
		if err := json.Unmarshal([]byte(s[i:end]), &key); err != nil || !isSensitiveKey(key) {
			i = end
			continue
		}
		k := skipSpace(s, j+1)
		out.WriteString(s[end:k])
		if k >= len(s) {
			i = k
			continue
		}
		vEnd := valueEnd(s, k)
		out.WriteString(`"` + redactPlaceholder(key) + `"`)
		i = vEnd
	}
	return out.String()
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

// stringEnd returns the index just past the string that starts at s[i]; an
// unterminated string runs to the end of s.
func stringEnd(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(s)
}

// valueEnd returns the index just past the JSON value that starts at s[i].
func valueEnd(s string, i int) int {
	switch s[i] {
	case '"':
		return stringEnd(s, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(s); j++ {
			switch s[j] {
			case '"':
				j = stringEnd(s, j) - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return j + 1
				}
			}
		}
		return len(s)
	default:
		j := i
		for j < len(s) && s[j] != ',' && s[j] != '}' && s[j] != ']' && s[j] != ' ' && s[j] != '\n' && s[j] != '\r' && s[j] != '\t' {
			j++
		}
		return j
	}
}
