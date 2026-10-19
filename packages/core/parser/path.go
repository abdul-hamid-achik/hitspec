package parser

import (
	"fmt"
	"strings"
)

// This file converts the body paths users write in assertions and captures to
// gjson paths. Both assertions and captures call it, so a path means the same
// thing everywhere.

// gjsonSpecial are the characters gjson treats specially inside a key.
const gjsonSpecial = ".*?|#@\\!=<>%"

func escapeGJSONKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		if strings.ContainsRune(gjsonSpecial, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// BodyPathToGJSON converts a body path to a gjson path. The gjson language is
// accepted as is (items.#, items.#(id==2).name, a|b, @reverse); on top of it
// bracket notation works: items[0].name, [2].id, items[*].id and
// a["b.c"].d. Brackets inside a #( ... ) query are left alone.
func BodyPathToGJSON(path string) string {
	var b strings.Builder
	depth := 0
	var quote byte
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case quote != 0:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(path) {
				i++
				b.WriteByte(path[i])
			} else if c == quote {
				quote = 0
			}
		case depth > 0:
			b.WriteByte(c)
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			case '"', '\'':
				quote = c
			}
		case c == '(':
			depth++
			b.WriteByte(c)
		case c == '[':
			j := strings.IndexByte(path[i:], ']')
			if j < 0 {
				b.WriteString(path[i:])
				i = len(path)
				break
			}
			inner := path[i+1 : i+j]
			var seg string
			switch {
			case inner == "*":
				seg = "#"
			case isDigits(inner):
				seg = inner
			case len(inner) >= 2 && (inner[0] == '\'' || inner[0] == '"') && inner[len(inner)-1] == inner[0]:
				seg = escapeGJSONKey(inner[1 : len(inner)-1])
			default:
				b.WriteString(path[i : i+j+1])
				i += j
				continue
			}
			if s := b.String(); s != "" && !strings.HasSuffix(s, ".") {
				b.WriteByte('.')
			}
			b.WriteString(seg)
			i += j
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimPrefix(b.String(), ".")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// JSONPathToGJSON converts the supported JSONPath subset to a gjson path:
//
//	$            the whole body
//	$.a.b        object members
//	$.a[0]       array index
//	$.a[*].b     member of every element (a.#.b)
//	$['a.b']     quoted member name
//
// Recursive descent (..), filters (?()), slices and unions are not supported
// and are reported as errors rather than guessed at.
func JSONPathToGJSON(expr string) (string, error) {
	e := strings.TrimSpace(expr)
	if e == "" || e[0] != '$' {
		return "", fmt.Errorf("JSONPath %q must start with $", expr)
	}
	var segs []string
	i := 1
	for i < len(e) {
		switch e[i] {
		case '.':
			if i+1 < len(e) && e[i+1] == '.' {
				return "", fmt.Errorf("JSONPath %q: recursive descent (..) is not supported", expr)
			}
			j := i + 1
			for j < len(e) && e[j] != '.' && e[j] != '[' {
				j++
			}
			name := e[i+1 : j]
			if name == "" {
				return "", fmt.Errorf("JSONPath %q: empty member name", expr)
			}
			if name == "*" {
				return "", fmt.Errorf("JSONPath %q: the .* wildcard is not supported (use [*])", expr)
			}
			segs = append(segs, escapeGJSONKey(name))
			i = j
		case '[':
			j := strings.IndexByte(e[i:], ']')
			if j < 0 {
				return "", fmt.Errorf("JSONPath %q: missing ]", expr)
			}
			inner := strings.TrimSpace(e[i+1 : i+j])
			switch {
			case inner == "*":
				segs = append(segs, "#")
			case isDigits(inner):
				segs = append(segs, inner)
			case len(inner) >= 2 && (inner[0] == '\'' || inner[0] == '"') && inner[len(inner)-1] == inner[0] && !strings.ContainsAny(inner[1:len(inner)-1], `'",`):
				segs = append(segs, escapeGJSONKey(inner[1:len(inner)-1]))
			default:
				return "", fmt.Errorf("JSONPath %q: [%s] is not supported (only [n], [*] and ['name'])", expr, inner)
			}
			i += j + 1
		default:
			return "", fmt.Errorf("JSONPath %q: unexpected %q at position %d", expr, string(e[i]), i)
		}
	}
	// A trailing [*] selects the elements, which is the array itself; gjson's
	// trailing # would be the count.
	if n := len(segs); n > 0 && segs[n-1] == "#" {
		segs = segs[:n-1]
	}
	return strings.Join(segs, "."), nil
}
