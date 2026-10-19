package postman

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// scriptResult is what a Postman test script translates to.
type scriptResult struct {
	Assertions   []string // lines for a >>> block, without the `expect` prefix handled by caller
	Untranslated []string // statements that need a human
}

var (
	jsonVarDecl   = regexp.MustCompile(`^(?:const|let|var)\s+(\w+)\s*=\s*(?:pm\.response\.json\(\)|JSON\.parse\(\s*(?:pm\.response\.text\(\)|responseBody)\s*\))$`)
	pmTestStr     = `(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|` + "`[^`]*`" + `)`
	pmTestBlock   = regexp.MustCompile(`pm\.test\(\s*` + pmTestStr + `\s*,\s*(?:async\s*)?(?:function\s*\w*\s*\(\s*\)|\(\s*\)\s*=>)\s*\{`)
	pmTestArrow   = regexp.MustCompile(`(?m)pm\.test\(\s*` + pmTestStr + `\s*,\s*\(\s*\)\s*=>\s*([^{\n]*?)\)\s*;?\s*$`)
	lineComment   = regexp.MustCompile(`(?m)(^|[\s;{}()])//[^\n]*`)
	blockComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	legacyTest    = regexp.MustCompile(`^tests\[[^\]]+\]\s*=\s*(.+)$`)
	envGet        = regexp.MustCompile(`^pm\.(?:environment|collectionVariables|globals|variables)\.get\(\s*(?:"([^"]*)"|'([^']*)')\s*\)$`)
	statusTextMap = map[string]int{
		"OK": 200, "Created": 201, "Accepted": 202, "No Content": 204,
		"Bad Request": 400, "Unauthorized": 401, "Forbidden": 403, "Not Found": 404,
		"Conflict": 409, "Unprocessable Entity": 422, "Too Many Requests": 429,
		"Internal Server Error": 500, "Bad Gateway": 502, "Service Unavailable": 503,
	}
	statusShortcuts = map[string]string{
		"ok": "status == 200", "accepted": "status == 202", "badRequest": "status == 400",
		"unauthorized": "status == 401", "forbidden": "status == 403", "notFound": "status == 404",
		"rateLimited": "status == 429", "serverError": "status >= 500",
	}
	fillerWords = map[string]bool{
		"to": true, "be": true, "been": true, "is": true, "that": true, "which": true, "and": true,
		"has": true, "have": true, "with": true, "at": true, "of": true, "same": true, "but": true,
		"does": true, "still": true, "also": true,
	}
	validTypes = map[string]bool{"null": true, "boolean": true, "number": true, "string": true, "array": true, "object": true}
)

// translateScript converts the common Postman test idioms to hitspec
// assertions. Anything it does not recognise is returned verbatim so the
// caller can leave a TODO instead of inventing an assertion.
func translateScript(src string) scriptResult {
	var out scriptResult
	src = blockComment.ReplaceAllString(src, "")
	src = lineComment.ReplaceAllString(src, "$1")

	src = pmTestBlock.ReplaceAllString(src, "\n")
	src = pmTestArrow.ReplaceAllString(src, "$1;")

	jsonVars := map[string]bool{"jsonData": true, "responseJson": true}
	for _, stmt := range splitStatements(src) {
		if m := jsonVarDecl.FindStringSubmatch(stmt); m != nil {
			jsonVars[m[1]] = true
		}
	}

	for _, stmt := range splitStatements(src) {
		stmt = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stmt), ";"))
		switch {
		case stmt == "" || stmt == ")" || stmt == "}":
			continue
		case jsonVarDecl.MatchString(stmt), strings.HasPrefix(stmt, "console."):
			continue
		}
		if line, ok := translateStatement(stmt, jsonVars); ok {
			out.Assertions = append(out.Assertions, line...)
			continue
		}
		out.Untranslated = append(out.Untranslated, collapse(stmt))
	}
	return out
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// splitStatements cuts a script at ; and newlines that are outside brackets
// and quotes, and at block braces. A line starting with `.` continues the
// previous statement (method chains).
func splitStatements(src string) []string {
	var stmts []string
	var cur strings.Builder
	depth := 0
	var quote rune
	rs := []rune(src)
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if quote != 0 {
			cur.WriteRune(r)
			if r == '\\' && i+1 < len(rs) {
				i++
				cur.WriteRune(rs[i])
			} else if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
			cur.WriteRune(r)
		case '(', '[':
			depth++
			cur.WriteRune(r)
		case ')', ']':
			if depth > 0 {
				depth--
				cur.WriteRune(r)
			} else {
				// Stray closer of a callback, e.g. the ")" in "});".
				flush()
			}
		case '{':
			if depth == 0 {
				// A block opener: keep it with its header so `if (x) {` is
				// reported as one statement, then start a new one.
				flush()
			} else {
				cur.WriteRune(r)
			}
		case '}':
			if depth == 0 {
				flush()
			} else {
				cur.WriteRune(r)
			}
		case ';':
			if depth == 0 {
				flush()
			} else {
				cur.WriteRune(r)
			}
		case '\n', '\r':
			if depth > 0 {
				cur.WriteRune(' ')
				break
			}
			// Continue a chain when the next non-space character is '.'.
			j := i + 1
			for j < len(rs) && (rs[j] == ' ' || rs[j] == '\t' || rs[j] == '\n' || rs[j] == '\r') {
				j++
			}
			if j < len(rs) && rs[j] == '.' {
				break
			}
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return stmts
}

// call is one element of a chai chain: name or name(args).
type call struct {
	name string
	args string
	has  bool
}

// splitChain splits "to.not.eql(5)" into elements, respecting parentheses.
func splitChain(s string) []call {
	var calls []call
	depth := 0
	var quote rune
	start := 0
	emit := func(end int) {
		part := s[start:end]
		if part == "" {
			return
		}
		c := call{name: part}
		if p := strings.Index(part, "("); p >= 0 && strings.HasSuffix(part, ")") {
			c.name = part[:p]
			c.args = part[p+1 : len(part)-1]
			c.has = true
		}
		calls = append(calls, c)
	}
	for i, r := range s {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '.':
			if depth == 0 {
				emit(i)
				start = i + 1
			}
		}
	}
	emit(len(s))
	return calls
}

var (
	pathExpr   = regexp.MustCompile(`^[A-Za-z_$][\w$]*((?:\.[A-Za-z_$][\w$]*|\[\d+\])*)$`)
	numLiteral = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
)

// translateStatement returns assertion lines for a recognised statement.
func translateStatement(stmt string, jsonVars map[string]bool) ([]string, bool) {
	// pm.response.to.have.status(...), pm.response.to.be.ok, pm.response.to.have.header(...)
	if strings.HasPrefix(stmt, "pm.response.to.") {
		calls := splitChain(strings.TrimPrefix(stmt, "pm.response.to."))
		var names []call
		for _, c := range calls {
			if !fillerWords[c.name] {
				names = append(names, c)
			}
		}
		neg := false
		if len(names) > 0 && names[0].name == "not" {
			neg = true
			names = names[1:]
		}
		if len(names) != 1 {
			return nil, false
		}
		c := names[0]
		switch {
		case c.name == "status" && c.has:
			if neg {
				return nil, false
			}
			arg := strings.TrimSpace(c.args)
			if numLiteral.MatchString(arg) {
				return []string{"status == " + arg}, true
			}
			if s, ok := unquote(arg); ok {
				if code, ok := statusTextMap[s]; ok {
					return []string{"status == " + strconv.Itoa(code)}, true
				}
			}
		case c.name == "header" && c.has:
			if s, ok := unquote(strings.Split(c.args, ",")[0]); ok {
				if neg {
					return []string{"header " + s + " !exists"}, true
				}
				if parts := strings.SplitN(c.args, ",", 2); len(parts) == 2 {
					if v, ok := unquote(parts[1]); ok {
						return []string{fmt.Sprintf("header %s == %q", s, v)}, true
					}
					return nil, false
				}
				return []string{"header " + s + " exists"}, true
			}
		case c.name == "success" && !c.has && !neg:
			return []string{"status >= 200", "status < 300"}, true
		case c.name == "clientError" && !c.has && !neg:
			return []string{"status >= 400", "status < 500"}, true
		case !c.has && !neg:
			if line, ok := statusShortcuts[c.name]; ok {
				return []string{line}, true
			}
		}
		return nil, false
	}

	// Legacy: tests["name"] = <comparison>
	if m := legacyTest.FindStringSubmatch(stmt); m != nil {
		return translateComparison(strings.TrimSpace(m[1]), jsonVars)
	}

	rest, ok := strings.CutPrefix(stmt, "pm.expect(")
	if !ok {
		return nil, false
	}
	closeAt := matchingParen(rest)
	if closeAt < 0 || !strings.HasPrefix(rest[closeAt+1:], ".") {
		return nil, false
	}
	subj, tail := strings.TrimSpace(rest[:closeAt]), rest[closeAt+1:]
	hsSubject, ok := translateSubject(subj, jsonVars)
	if !ok {
		return nil, false
	}

	calls := splitChain(strings.TrimPrefix(tail, "."))
	neg := false
	var ops []call
	for _, c := range calls {
		switch {
		case c.name == "not":
			neg = !neg
		case fillerWords[c.name] && !c.has:
		default:
			ops = append(ops, c)
		}
	}
	if len(ops) != 1 {
		return nil, false
	}
	op := ops[0]
	arg := strings.TrimSpace(op.args)

	cmp := func(sym string) ([]string, bool) {
		if neg {
			return nil, false
		}
		v, ok := translateLiteral(arg)
		if !ok {
			return nil, false
		}
		return []string{hsSubject + " " + sym + " " + v}, true
	}

	switch op.name {
	case "eql", "eqls", "equal", "equals", "eq", "deepEqual":
		v, ok := translateLiteral(arg)
		if !ok || !op.has {
			return nil, false
		}
		sym := "=="
		if neg {
			sym = "!="
		}
		return []string{hsSubject + " " + sym + " " + v}, true
	case "above", "greaterThan", "gt":
		return cmp(">")
	case "least", "gte":
		return cmp(">=")
	case "below", "lessThan", "lt":
		return cmp("<")
	case "most", "lte":
		return cmp("<=")
	case "include", "includes", "contain", "contains", "string":
		if !op.has {
			return nil, false
		}
		v, ok := translateLiteral(arg)
		if !ok {
			return nil, false
		}
		sym := "contains"
		if neg {
			sym = "!contains"
		}
		return []string{hsSubject + " " + sym + " " + v}, true
	case "exist":
		if op.has {
			return nil, false
		}
		if neg {
			return []string{hsSubject + " !exists"}, true
		}
		return []string{hsSubject + " exists"}, true
	case "undefined":
		if neg {
			return []string{hsSubject + " exists"}, true
		}
		return []string{hsSubject + " !exists"}, true
	case "null", "true", "false":
		if op.has {
			return nil, false
		}
		sym := "=="
		if neg {
			sym = "!="
		}
		return []string{hsSubject + " " + sym + " " + op.name}, true
	case "a", "an":
		t, ok := unquote(arg)
		if !ok || !validTypes[t] || neg {
			return nil, false
		}
		return []string{hsSubject + " type " + t}, true
	case "property":
		name, ok := unquote(strings.Split(arg, ",")[0])
		if !ok || strings.Contains(arg, ",") || !pathSegment.MatchString(name) {
			return nil, false
		}
		sym := "exists"
		if neg {
			sym = "!exists"
		}
		return []string{hsSubject + "." + name + " " + sym}, true
	case "length", "lengthOf":
		if !op.has || neg || !numLiteral.MatchString(arg) {
			return nil, false
		}
		return []string{hsSubject + " length " + arg}, true
	case "match", "matches":
		if neg || !strings.HasPrefix(arg, "/") {
			return nil, false
		}
		return []string{hsSubject + " matches " + arg}, true
	case "oneOf":
		if !strings.HasPrefix(arg, "[") || !strings.HasSuffix(arg, "]") {
			return nil, false
		}
		items := splitTop(arg[1 : len(arg)-1])
		var lits []string
		for _, it := range items {
			v, ok := translateLiteral(strings.TrimSpace(it))
			if !ok {
				return nil, false
			}
			lits = append(lits, v)
		}
		sym := "in"
		if neg {
			sym = "!in"
		}
		return []string{hsSubject + " " + sym + " [" + strings.Join(lits, ", ") + "]"}, true
	}
	return nil, false
}

var pathSegment = regexp.MustCompile(`^[A-Za-z_][\w-]*$`)

// matchingParen returns the index of the ')' closing an already-open '(' in s.
func matchingParen(s string) int {
	depth := 1
	var quote rune
	for i, r := range s {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// translateSubject maps the argument of pm.expect(...) to an assertion
// subject.
func translateSubject(subj string, jsonVars map[string]bool) (string, bool) {
	switch subj {
	case "pm.response.code", "responseCode.code":
		return "status", true
	case "pm.response.responseTime", "responseTime":
		return "duration", true
	case "pm.response.text()", "responseBody":
		return "body", true
	case "pm.response.json()":
		return "body", true
	}
	if rest, ok := strings.CutPrefix(subj, "pm.response.headers.get("); ok && strings.HasSuffix(rest, ")") {
		if name, ok := unquote(rest[:len(rest)-1]); ok {
			return "header " + name, true
		}
		return "", false
	}
	if rest, ok := strings.CutPrefix(subj, "pm.response.json()"); ok {
		subj = "jsonData" + rest
		jsonVars = map[string]bool{"jsonData": true}
	}
	if m := pathExpr.FindStringSubmatch(subj); m != nil {
		root := subj[:len(subj)-len(m[1])]
		if jsonVars[root] {
			return "body" + m[1], true
		}
	}
	return "", false
}

var comparison = regexp.MustCompile(`^(.+?)\s*(===|==|!==|!=|<=|>=|<|>)\s*(.+)$`)

// translateComparison handles legacy `tests[..] = a === b` expressions.
func translateComparison(expr string, jsonVars map[string]bool) ([]string, bool) {
	m := comparison.FindStringSubmatch(expr)
	if m == nil {
		return nil, false
	}
	subj, ok := translateSubject(strings.TrimSpace(m[1]), jsonVars)
	if !ok {
		return nil, false
	}
	v, ok := translateLiteral(strings.TrimSpace(m[3]))
	if !ok {
		return nil, false
	}
	sym := m[2]
	switch sym {
	case "===":
		sym = "=="
	case "!==":
		sym = "!="
	}
	return []string{subj + " " + sym + " " + v}, true
}

// translateLiteral converts a JS literal to an assertion value.
func translateLiteral(s string) (string, bool) {
	s = strings.TrimSpace(s)
	switch {
	case s == "true" || s == "false" || s == "null":
		return s, true
	case numLiteral.MatchString(s):
		return s, true
	}
	if v, ok := unquote(s); ok {
		return strconv.Quote(v), true
	}
	if m := envGet.FindStringSubmatch(s); m != nil {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		return "{{" + name + "}}", true
	}
	return "", false
}

// unquote strips one pair of JS string quotes. Strings with escapes or
// interpolation are not translated.
func unquote(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return "", false
	}
	q := s[0]
	if (q != '"' && q != '\'' && q != '`') || s[len(s)-1] != q {
		return "", false
	}
	body := s[1 : len(s)-1]
	if strings.ContainsRune(body, rune(q)) || strings.Contains(body, "\\") || (q == '`' && strings.Contains(body, "${")) {
		return "", false
	}
	return body, true
}

// splitTop splits on commas outside brackets and quotes.
func splitTop(s string) []string {
	var parts []string
	depth := 0
	var quote rune
	start := 0
	for i, r := range s {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}
