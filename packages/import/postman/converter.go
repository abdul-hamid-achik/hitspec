// Package postman converts Postman Collection v2.x (and environment) exports
// to hitspec .http files.
package postman

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/result"
)

// Converter converts Postman exports to hitspec format.
type Converter struct{}

// NewConverter creates a Postman converter.
func NewConverter() *Converter { return &Converter{} }

// ConvertFile converts a Postman collection or environment file.
func (c *Converter) ConvertFile(path string) (*result.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return c.Convert(data)
}

// Convert converts a Postman collection (or environment export) to hitspec.
//
// Variables: a collection's `variable` list becomes `@name = value` lines at
// the top of the file. A Postman environment export becomes a hitspec.yaml
// `environments:` entry in Result.Environments, because per-environment values
// must not be pinned by `@var` lines (file variables override environments).
func (c *Converter) Convert(data []byte) (*result.Result, error) {
	var col collection
	if err := json.Unmarshal(data, &col); err != nil {
		return nil, fmt.Errorf("failed to parse Postman collection: %w", err)
	}

	res := &result.Result{}

	// An environment export has `values` and no `item`.
	if len(col.Item) == 0 && len(col.Values) > 0 {
		name := envName(col.Name)
		vars := map[string]any{}
		for _, v := range col.Values {
			if v.Enabled != nil && !*v.Enabled {
				continue
			}
			vars[v.Key] = convertVariable(res, result.Scalar(v.Value))
		}
		res.Environments = map[string]map[string]any{name: vars}
		res.DefaultEnvironment = name
		// No requests: the environments section is the whole product.
		return res, nil
	}

	var sb strings.Builder
	sb.WriteString("# Generated from Postman collection")
	if col.Info.Name != "" {
		sb.WriteString(": ")
		sb.WriteString(oneLine(col.Info.Name))
	}
	sb.WriteString("\n\n")

	// Collection variables.
	wroteVar := false
	for _, v := range col.Variable {
		if v.Key == "" || v.Disabled {
			continue
		}
		fmt.Fprintf(&sb, "@%s = %s\n", v.Key, oneLine(convertVariable(res, result.Scalar(v.Value))))
		wroteVar = true
	}
	if wroteVar {
		sb.WriteString("\n")
	}

	w := &writer{sb: &sb, res: res, names: map[string]int{}}
	inherited := scope{auth: col.Auth, events: col.Event}
	w.items(col.Item, "", inherited)

	res.Content = sb.String()
	return res, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

var envNameBad = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func envName(n string) string {
	n = strings.Trim(envNameBad.ReplaceAllString(strings.ToLower(n), "-"), "-")
	if n == "" {
		return "postman"
	}
	return n
}

// scope carries what folders and the collection pass down to requests.
type scope struct {
	auth   *auth
	events []event
}

type writer struct {
	sb    *strings.Builder
	res   *result.Result
	names map[string]int
}

func (w *writer) items(items []item, prefix string, sc scope) {
	for _, it := range items {
		if len(it.Item) > 0 {
			next := sc
			if it.Auth != nil {
				next.auth = it.Auth
			}
			next.events = append(append([]event{}, sc.events...), it.Event...)
			p := it.Name
			if prefix != "" {
				p = prefix + "/" + it.Name
			}
			w.items(it.Item, p, next)
			continue
		}
		if len(it.Request) == 0 {
			continue
		}
		w.request(it, prefix, sc)
	}
}

func (w *writer) request(it item, prefix string, sc scope) {
	req, err := parseRequest(it.Request)
	if err != nil {
		w.res.Warn("request %q skipped: %v", it.Name, err)
		return
	}
	var notes []string
	note := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		notes = append(notes, msg)
		w.res.Warn("%s: %s", it.Name, msg)
	}

	title := it.Name
	if prefix != "" {
		title = prefix + " - " + it.Name
	}
	name := sanitizeName(it.Name)
	if name == "" {
		name = "request"
	}
	w.names[name]++
	if n := w.names[name]; n > 1 {
		name = fmt.Sprintf("%s_%d", name, n)
	}

	// Auth: request > folder > collection.
	effective := req.Auth
	if effective == nil {
		effective = sc.auth
	}
	authLine, authHeader, authNote := convertAuth(w.res, effective)
	if authNote != "" {
		note("%s", authNote)
	}

	// Method and URL.
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "GET"
	}
	reqURL := w.res2url(req.URL, note)

	// Headers.
	var headers [][2]string
	hasHeader := func(k string) bool {
		for _, h := range headers {
			if strings.EqualFold(h[0], k) {
				return true
			}
		}
		return false
	}
	for _, h := range req.Header {
		if h.Disabled || h.Key == "" {
			continue
		}
		headers = append(headers, [2]string{h.Key, oneLine(convertVariable(w.res, h.Value))})
	}
	if authHeader != nil && !hasHeader(authHeader[0]) {
		headers = append(headers, *authHeader)
	}

	// Body.
	body := w.body(req.Body, &headers, hasHeader, note)

	// Tests, with folder/collection scripts applied to every request.
	var assertions, untranslated []string
	events := append(append([]event{}, sc.events...), it.Event...)
	for _, ev := range events {
		script := strings.Join(ev.Script.lines(), "\n")
		if strings.TrimSpace(script) == "" {
			continue
		}
		switch ev.Listen {
		case "test":
			tr := translateScript(script)
			assertions = append(assertions, tr.Assertions...)
			untranslated = append(untranslated, tr.Untranslated...)
		case "prerequest":
			for _, st := range splitStatements(script) {
				if s := collapse(st); s != "" && s != ")" {
					note("untranslated pre-request script: %s", s)
				}
			}
		}
	}
	for _, u := range untranslated {
		note("untranslated test: %s", u)
	}
	assertions = dedupe(assertions)

	sb := w.sb
	sb.WriteString("### ")
	sb.WriteString(oneLine(title))
	sb.WriteString("\n# @name ")
	sb.WriteString(name)
	sb.WriteString("\n")
	for _, n := range notes {
		sb.WriteString("# TODO: ")
		sb.WriteString(oneLine(n))
		sb.WriteString("\n")
	}
	if authLine != "" {
		sb.WriteString(authLine)
		sb.WriteString("\n")
	}
	sb.WriteString(method)
	sb.WriteString(" ")
	sb.WriteString(reqURL)
	sb.WriteString("\n")
	for _, h := range headers {
		sb.WriteString(h[0])
		sb.WriteString(": ")
		sb.WriteString(h[1])
		sb.WriteString("\n")
	}
	if body != "" {
		sb.WriteString("\n")
		sb.WriteString(body)
		sb.WriteString("\n")
	}
	if len(assertions) > 0 {
		sb.WriteString("\n>>>\n")
		for _, a := range assertions {
			sb.WriteString("expect ")
			sb.WriteString(a)
			sb.WriteString("\n")
		}
		sb.WriteString("<<<\n")
	}
	sb.WriteString("\n")
}

// res2url renders a request URL, substituting Postman path variables.
func (w *writer) res2url(u postmanURL, note func(string, ...any)) string {
	raw := u.Raw
	if raw == "" {
		raw = u.build()
	}
	// :id path variables are Postman-only; hitspec has no such syntax.
	if len(u.Variable) > 0 {
		pathPart, query, hasQuery := strings.Cut(raw, "?")
		for _, v := range u.Variable {
			if v.Key == "" {
				continue
			}
			val := result.Scalar(v.Value)
			if val == "" {
				val = "{{" + v.Key + "}}"
			}
			pathPart = strings.ReplaceAll(pathPart, "/:"+v.Key, "/"+val)
		}
		raw = pathPart
		if hasQuery {
			raw += "?" + query
		}
	}
	return oneLine(convertVariable(w.res, raw))
}

func (w *writer) body(b *postmanBody, headers *[][2]string, hasHeader func(string) bool, note func(string, ...any)) string {
	if b == nil || b.Mode == "" {
		return ""
	}
	if b.Disabled {
		return ""
	}
	switch b.Mode {
	case "raw":
		if b.Raw == "" {
			return ""
		}
		if lang := b.Options.Raw.Language; lang == "json" && !hasHeader("Content-Type") {
			*headers = append(*headers, [2]string{"Content-Type", "application/json"})
		}
		return convertVariable(w.res, b.Raw)
	case "urlencoded":
		var parts []string
		for _, kv := range b.URLEncoded {
			if kv.Disabled || kv.Key == "" {
				continue
			}
			parts = append(parts, queryEscape(convertVariable(w.res, kv.Key))+"="+queryEscape(convertVariable(w.res, kv.Value)))
		}
		return strings.Join(parts, "&")
	case "formdata":
		var sb strings.Builder
		sb.WriteString(">>>multipart\n")
		n := 0
		for _, f := range b.FormData {
			if f.Disabled || f.Key == "" {
				continue
			}
			n++
			if f.Type == "file" {
				src := f.srcPath()
				if src == "" {
					note("form file field %q has no file path; add `file @./path` by hand", f.Key)
					continue
				}
				fmt.Fprintf(&sb, "%s\n", result.MultipartFileLine(f.Key, relativePath(src)))
				if !result.CanNameMultipartFile(f.Key) {
					note("multipart file field name %q cannot be written as `file <name> @path` (it has spaces); the part is sent as field \"file\", set the name by hand", f.Key)
				}
				continue
			}
			fmt.Fprintf(&sb, "field %s = %s\n", f.Key, oneLine(convertVariable(w.res, f.Value)))
		}
		sb.WriteString("<<<")
		if n == 0 {
			return ""
		}
		return sb.String()
	case "graphql":
		if b.GraphQL == nil {
			return ""
		}
		if !hasHeader("Content-Type") {
			*headers = append(*headers, [2]string{"Content-Type", "application/json"})
		}
		var sb strings.Builder
		sb.WriteString(">>>graphql\n")
		sb.WriteString(strings.TrimRight(convertVariable(w.res, b.GraphQL.Query), "\n"))
		sb.WriteString("\n")
		if v := strings.TrimSpace(b.GraphQL.variables()); v != "" {
			sb.WriteString("\n>>>variables\n")
			sb.WriteString(convertVariable(w.res, v))
			sb.WriteString("\n")
		}
		sb.WriteString("<<<")
		return sb.String()
	case "file":
		if b.File != nil && b.File.Src != "" {
			return "< " + relativePath(b.File.Src)
		}
		note("binary file body has no path; add `< ./path` by hand")
		return ""
	}
	note("body mode %q is not supported and was dropped", b.Mode)
	return ""
}

// convertAuth returns the `# @auth` line (or a fallback header) for an auth
// block and a note when something could not be carried over.
func convertAuth(res *result.Result, a *auth) (line string, header *[2]string, note string) {
	if a == nil || a.Type == "" || a.Type == "noauth" {
		return "", nil, ""
	}
	p := a.params()
	get := func(k string) string { return convertVariable(res, p[k]) }
	switch a.Type {
	case "bearer":
		tok := get("token")
		if tok == "" {
			return "", nil, "bearer auth has no token"
		}
		if hasBareComma(tok) {
			return "", &[2]string{"Authorization", "Bearer " + tok}, ""
		}
		return "# @auth bearer " + oneLine(tok), nil, ""
	case "basic":
		user, pass := get("username"), get("password")
		if user == "" {
			return "", nil, "basic auth has no username"
		}
		if pass == "" || hasBareComma(user) || hasBareComma(pass) {
			if strings.Contains(user+pass, "{{") {
				return "", nil, "basic auth credentials contain a comma or are empty; add `# @auth basic user, password` by hand"
			}
			tok := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
			return "", &[2]string{"Authorization", "Basic " + tok}, ""
		}
		return fmt.Sprintf("# @auth basic %s, %s", oneLine(user), oneLine(pass)), nil, ""
	case "digest":
		user, pass := get("username"), get("password")
		if user == "" || pass == "" || hasBareComma(user) || hasBareComma(pass) {
			return "", nil, "digest auth credentials are missing or contain a comma; add `# @auth digest user, password` by hand"
		}
		return fmt.Sprintf("# @auth digest %s, %s", oneLine(user), oneLine(pass)), nil, ""
	case "apikey":
		key, val := get("key"), get("value")
		if key == "" || val == "" || hasBareComma(key) || hasBareComma(val) {
			return "", nil, "API key auth is incomplete or contains a comma; add `# @auth apiKey name, value` by hand"
		}
		kind := "apiKey"
		if p["in"] == "query" {
			kind = "apiKeyQuery"
		}
		return fmt.Sprintf("# @auth %s %s, %s", kind, oneLine(key), oneLine(val)), nil, ""
	}
	return "", nil, fmt.Sprintf("auth type %q is not supported and was dropped", a.Type)
}

// hasBareComma reports a comma outside {{ }}, which the @auth parser would
// read as an argument separator.
func hasBareComma(s string) bool {
	return strings.Contains(varRef.ReplaceAllString(s, ""), ",")
}

var (
	varRef     = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	dynamicVar = regexp.MustCompile(`\{\{\$(\w+)\}\}`)
)

// dynamicVars maps Postman's dynamic variables to hitspec built-ins.
var dynamicVars = map[string]string{
	"guid": "{{$uuid()}}", "randomUUID": "{{$uuid()}}",
	"timestamp":   "{{$timestamp()}}",
	"randomInt":   "{{$random(0, 1000)}}",
	"randomEmail": "{{$randomEmail()}}",
}

// convertVariable rewrites Postman dynamic variables; plain {{name}} is the
// same in both tools. Unknown dynamic variables are left in place and reported.
func convertVariable(res *result.Result, s string) string {
	return dynamicVar.ReplaceAllStringFunc(s, func(m string) string {
		name := dynamicVar.FindStringSubmatch(m)[1]
		if repl, ok := dynamicVars[name]; ok {
			return repl
		}
		res.Warn("Postman dynamic variable {{$%s}} has no hitspec equivalent and was left as is", name)
		return m
	})
}

func queryEscape(s string) string {
	var sb strings.Builder
	last := 0
	for _, loc := range varRef.FindAllStringIndex(s, -1) {
		sb.WriteString(url.QueryEscape(s[last:loc[0]]))
		sb.WriteString(s[loc[0]:loc[1]])
		last = loc[1]
	}
	sb.WriteString(url.QueryEscape(s[last:]))
	return sb.String()
}

func relativePath(p string) string {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "{{") {
		return p
	}
	return "./" + p
}

// oneLine collapses newlines so a value cannot break out of its header line.
func oneLine(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
}

// sanitizeName turns a request name into an @name identifier.
func sanitizeName(name string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}
