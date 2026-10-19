// Package insomnia provides Insomnia export to hitspec format conversion.
package insomnia

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/result"
)

// Converter converts Insomnia exports to hitspec format.
type Converter struct {
	generateAssertions bool
}

// Option is a functional option for Converter.
type Option func(*Converter)

// WithAssertions configures whether to generate test assertions.
func WithAssertions(generate bool) Option {
	return func(c *Converter) {
		c.generateAssertions = generate
	}
}

// NewConverter creates a new Insomnia converter.
func NewConverter(opts ...Option) *Converter {
	c := &Converter{
		generateAssertions: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Export represents an Insomnia export file.
type Export struct {
	Type         string     `json:"_type"`
	ExportFormat int        `json:"__export_format"`
	Resources    []Resource `json:"resources"`
}

// Resource represents an Insomnia resource (request, folder, environment, etc).
type Resource struct {
	ID             string         `json:"_id"`
	Type           string         `json:"_type"`
	ParentID       string         `json:"parentId"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Method         string         `json:"method,omitempty"`
	URL            string         `json:"url,omitempty"`
	Headers        []Header       `json:"headers,omitempty"`
	Body           *Body          `json:"body,omitempty"`
	Parameters     []Parameter    `json:"parameters,omitempty"`
	Authentication *Auth          `json:"authentication,omitempty"`
	Data           map[string]any `json:"data,omitempty"` // environments
}

// Header represents an Insomnia header.
type Header struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Body represents an Insomnia request body.
type Body struct {
	MimeType string      `json:"mimeType,omitempty"`
	Text     string      `json:"text,omitempty"`
	Params   []BodyParam `json:"params,omitempty"`
	FileName string      `json:"fileName,omitempty"`
}

// BodyParam is a form-urlencoded or multipart entry.
type BodyParam struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Type     string `json:"type,omitempty"` // "file" for multipart files
	FileName string `json:"fileName,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Parameter represents an Insomnia query parameter.
type Parameter struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Auth represents Insomnia authentication.
type Auth struct {
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
	AddTo    string `json:"addTo,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// ConvertFile converts an Insomnia export file to hitspec format. Environments
// that cannot live in the .http file are appended as a commented hitspec.yaml
// block; use ConvertFileResult to get them as data.
func (c *Converter) ConvertFile(path string) (string, error) {
	res, err := c.ConvertFileResult(path)
	if err != nil {
		return "", err
	}
	return res.ContentWithEnvironments(), nil
}

// ConvertFileResult converts an Insomnia export file and reports warnings and
// environments.
func (c *Converter) ConvertFileResult(path string) (*result.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return c.ConvertResult(data)
}

// Convert converts Insomnia export JSON to hitspec format. See ConvertFile for
// how environments are returned.
func (c *Converter) Convert(data []byte) (string, error) {
	res, err := c.ConvertResult(data)
	if err != nil {
		return "", err
	}
	return res.ContentWithEnvironments(), nil
}

// ConvertResult converts Insomnia export JSON.
//
// Environments: Insomnia's base environment becomes `@name = value` lines in
// the .http file. When sub environments exist, every variable moves to the
// hitspec.yaml `environments:` section instead (base and each sub environment
// merged over it), because `@var` lines in a file override the selected
// environment and would defeat the per-environment values.
func (c *Converter) ConvertResult(data []byte) (*result.Result, error) {
	var export Export
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("failed to parse Insomnia export: %w", err)
	}

	requests := make([]Resource, 0)
	folders := make(map[string]Resource)
	var envs []Resource

	for _, res := range export.Resources {
		switch res.Type {
		case "request":
			requests = append(requests, res)
		case "request_group":
			folders[res.ID] = res
		case "environment":
			envs = append(envs, res)
		}
	}

	res := &result.Result{}
	var sb strings.Builder
	sb.WriteString("# Generated from Insomnia export\n\n")

	c.writeEnvironments(&sb, res, envs)
	w := &writer{c: c, sb: &sb, res: res, names: map[string]int{}}
	w.requests(requests, folders)

	res.Content = sb.String()
	if strings.Contains(res.Content, "{%") {
		res.Warn("Insomnia template tags ({%% ... %%}) have no hitspec equivalent and were left as is")
	}
	return res, nil
}

// writeEnvironments imports Insomnia environments (see ConvertResult).
func (c *Converter) writeEnvironments(sb *strings.Builder, res *result.Result, envs []Resource) {
	if len(envs) == 0 {
		return
	}
	byID := map[string]Resource{}
	for _, e := range envs {
		byID[e.ID] = e
	}
	var base *Resource
	var subs []Resource
	for i, e := range envs {
		if _, parentIsEnv := byID[e.ParentID]; parentIsEnv {
			subs = append(subs, e)
		} else if base == nil {
			base = &envs[i]
		} else {
			subs = append(subs, e)
		}
	}

	flat := func(e Resource) map[string]any {
		out := map[string]any{}
		for k, v := range e.Data {
			switch t := v.(type) {
			case string:
				out[k] = c.convertVariable(t)
			case float64:
				out[k] = result.Normalize(t)
			case bool:
				out[k] = t
			case nil:
				out[k] = ""
			default:
				b, _ := json.Marshal(t)
				out[k] = string(b)
				res.Warn("environment variable %q in %q is an object/array; stored as a JSON string", k, e.Name)
			}
		}
		return out
	}

	if len(subs) == 0 {
		if base == nil {
			return
		}
		vars := flat(*base)
		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(sb, "@%s = %s\n", k, oneLine(result.Scalar(vars[k])))
		}
		if len(keys) > 0 {
			sb.WriteString("\n")
		}
		return
	}

	res.Environments = map[string]map[string]any{}
	baseVars := map[string]any{}
	if base != nil {
		baseVars = flat(*base)
	}
	res.Environments["base"] = baseVars
	res.DefaultEnvironment = "base"
	for _, e := range subs {
		name := envName(e.Name)
		for _, taken := res.Environments[name]; taken; _, taken = res.Environments[name] {
			name += "-2"
		}
		merged := map[string]any{}
		for k, v := range baseVars {
			merged[k] = v
		}
		for k, v := range flat(e) {
			merged[k] = v
		}
		res.Environments[name] = merged
	}
	fmt.Fprintf(sb, "# Environment variables are in the hitspec.yaml environments section (%s); select one with --env.\n\n", strings.Join(res.EnvironmentNames(), ", "))
}

var envNameBad = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func envName(n string) string {
	n = strings.Trim(envNameBad.ReplaceAllString(strings.ToLower(n), "-"), "-")
	if n == "" {
		return "env"
	}
	return n
}

type writer struct {
	c     *Converter
	sb    *strings.Builder
	res   *result.Result
	names map[string]int
}

func oneLine(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
}

func (w *writer) requests(requests []Resource, folders map[string]Resource) {
	c, sb := w.c, w.sb
	for _, req := range requests {
		var notes []string
		note := func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			notes = append(notes, msg)
			w.res.Warn("%s: %s", req.Name, msg)
		}

		// Get folder path
		folderPath := c.getFolderPath(req.ParentID, folders)

		name := sanitizeName(req.Name)
		if name == "" {
			name = "request"
		}
		w.names[name]++
		if n := w.names[name]; n > 1 {
			name = fmt.Sprintf("%s_%d", name, n)
		}

		// Request separator
		sb.WriteString("### ")
		if folderPath != "" {
			sb.WriteString(folderPath)
			sb.WriteString(" - ")
		}
		sb.WriteString(oneLine(req.Name))
		sb.WriteString("\n")

		// Name annotation
		sb.WriteString("# @name ")
		sb.WriteString(name)
		sb.WriteString("\n")

		// Description
		if req.Description != "" {
			sb.WriteString("# @description ")
			sb.WriteString(oneLine(req.Description))
			sb.WriteString("\n")
		}

		// Headers (kept so auth/body can add to them)
		var headers [][2]string
		hasHeader := func(k string) bool {
			for _, h := range headers {
				if strings.EqualFold(h[0], k) {
					return true
				}
			}
			return false
		}
		for _, h := range req.Headers {
			if h.Disabled || h.Name == "" {
				continue
			}
			headers = append(headers, [2]string{h.Name, oneLine(c.convertVariable(h.Value))})
		}

		// Auth annotation
		var authLine string
		if req.Authentication != nil {
			var h *[2]string
			authLine, h = c.authLine(req.Authentication, note)
			if h != nil && !hasHeader(h[0]) {
				headers = append(headers, *h)
			}
		}

		body := w.body(req.Body, &headers, hasHeader, note)

		for _, n := range notes {
			sb.WriteString("# TODO: ")
			sb.WriteString(oneLine(n))
			sb.WriteString("\n")
		}
		if authLine != "" {
			sb.WriteString(authLine)
			sb.WriteString("\n")
		}

		// Method and URL
		method := req.Method
		if method == "" {
			method = "GET"
		}
		sb.WriteString(method)
		sb.WriteString(" ")
		sb.WriteString(oneLine(c.convertURL(req.URL)))
		sb.WriteString("\n")

		// Query parameters (as separate lines)
		for _, param := range req.Parameters {
			if param.Disabled {
				continue
			}
			sb.WriteString("? ")
			sb.WriteString(param.Name)
			sb.WriteString(" = ")
			sb.WriteString(oneLine(c.convertVariable(param.Value)))
			sb.WriteString("\n")
		}

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

		// Assertions
		if c.generateAssertions {
			sb.WriteString("\n>>>\n")
			sb.WriteString("expect status >= 200\n")
			sb.WriteString("expect status < 400\n")
			sb.WriteString("<<<\n")
		}

		sb.WriteString("\n")
	}
}

// body renders a request body. Form, multipart, GraphQL and file bodies were
// previously dropped silently because only Body.Text was read.
func (w *writer) body(b *Body, headers *[][2]string, hasHeader func(string) bool, note func(string, ...any)) string {
	if b == nil {
		return ""
	}
	c := w.c
	setCT := func(ct string) {
		if !hasHeader("Content-Type") {
			*headers = append(*headers, [2]string{"Content-Type", ct})
		}
	}
	mime := strings.ToLower(b.MimeType)
	switch {
	case strings.HasPrefix(mime, "multipart/form-data"):
		var sb strings.Builder
		sb.WriteString(">>>multipart\n")
		n := 0
		for _, p := range b.Params {
			if p.Disabled || p.Name == "" {
				continue
			}
			n++
			if p.Type == "file" {
				src := p.FileName
				if src == "" {
					note("form file field %q has no file path; add `file @./path` by hand", p.Name)
					continue
				}
				fmt.Fprintf(&sb, "file @%s\n", relativePath(src))
				note("multipart file field name %q cannot be expressed by hitspec's `file @path` line; set the name by hand", p.Name)
				continue
			}
			fmt.Fprintf(&sb, "field %s = %s\n", p.Name, oneLine(c.convertVariable(p.Value)))
		}
		sb.WriteString("<<<")
		if n == 0 {
			return ""
		}
		return sb.String()
	case strings.HasPrefix(mime, "application/x-www-form-urlencoded"):
		var parts []string
		for _, p := range b.Params {
			if p.Disabled || p.Name == "" {
				continue
			}
			parts = append(parts, queryEscape(c.convertVariable(p.Name))+"="+queryEscape(c.convertVariable(p.Value)))
		}
		setCT("application/x-www-form-urlencoded")
		return strings.Join(parts, "&")
	case strings.HasPrefix(mime, "application/graphql"):
		var gql struct {
			Query     string          `json:"query"`
			Variables json.RawMessage `json:"variables"`
		}
		if json.Unmarshal([]byte(b.Text), &gql) != nil || gql.Query == "" {
			setCT("application/json")
			return c.convertVariable(b.Text)
		}
		setCT("application/json")
		var sb strings.Builder
		sb.WriteString(">>>graphql\n")
		sb.WriteString(strings.TrimRight(c.convertVariable(gql.Query), "\n"))
		sb.WriteString("\n")
		if v := strings.TrimSpace(string(gql.Variables)); v != "" && v != "null" && v != "{}" {
			sb.WriteString("\n>>>variables\n")
			sb.WriteString(c.convertVariable(v))
			sb.WriteString("\n")
		}
		sb.WriteString("<<<")
		return sb.String()
	case b.FileName != "" && b.Text == "":
		if mime != "" {
			setCT(b.MimeType)
		}
		return "< " + relativePath(b.FileName)
	}
	if b.Text == "" {
		if len(b.Params) > 0 {
			note("body with MIME type %q has parameters that were dropped", b.MimeType)
		}
		return ""
	}
	if mime != "" {
		setCT(b.MimeType)
	}
	return c.convertVariable(b.Text)
}

var varRef = regexp.MustCompile(`\{\{[^{}]*\}\}`)

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

func hasBareComma(s string) bool {
	return strings.Contains(varRef.ReplaceAllString(s, ""), ",")
}

// authLine renders an Insomnia auth block as an @auth annotation, or as a
// header when the values cannot be written in the comma syntax.
func (c *Converter) authLine(a *Auth, note func(string, ...any)) (string, *[2]string) {
	if a.Disabled || a.Type == "" || a.Type == "none" {
		return "", nil
	}
	switch a.Type {
	case "basic":
		user, pass := c.convertVariable(a.Username), c.convertVariable(a.Password)
		if user == "" {
			return "", nil
		}
		if pass == "" || hasBareComma(user) || hasBareComma(pass) {
			if strings.Contains(user+pass, "{{") {
				note("basic auth credentials contain a comma or are empty; add `# @auth basic user, password` by hand")
				return "", nil
			}
			tok := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
			return "", &[2]string{"Authorization", "Basic " + tok}
		}
		return fmt.Sprintf("# @auth basic %s, %s", oneLine(user), oneLine(pass)), nil
	case "bearer":
		tok := c.convertVariable(a.Token)
		if tok == "" {
			return "", nil
		}
		if hasBareComma(tok) {
			return "", &[2]string{"Authorization", "Bearer " + tok}
		}
		return "# @auth bearer " + oneLine(tok), nil
	case "digest":
		user, pass := c.convertVariable(a.Username), c.convertVariable(a.Password)
		if user == "" || pass == "" || hasBareComma(user) || hasBareComma(pass) {
			note("digest auth credentials are missing or contain a comma; add `# @auth digest user, password` by hand")
			return "", nil
		}
		return fmt.Sprintf("# @auth digest %s, %s", oneLine(user), oneLine(pass)), nil
	case "apikey":
		key, val := c.convertVariable(a.Key), c.convertVariable(a.Value)
		if key == "" || val == "" || hasBareComma(key) || hasBareComma(val) {
			note("API key auth is incomplete or contains a comma; add `# @auth apiKey name, value` by hand")
			return "", nil
		}
		kind := "apiKey"
		if a.AddTo == "queryParams" {
			kind = "apiKeyQuery"
		}
		return fmt.Sprintf("# @auth %s %s, %s", kind, oneLine(key), oneLine(val)), nil
	}
	note("auth type %q is not supported and was dropped", a.Type)
	return "", nil
}

func (c *Converter) getFolderPath(parentID string, folders map[string]Resource) string {
	var path []string
	currentID := parentID

	for {
		folder, exists := folders[currentID]
		if !exists {
			break
		}
		path = append([]string{folder.Name}, path...)
		currentID = folder.ParentID
	}

	return strings.Join(path, "/")
}

func (c *Converter) convertURL(url string) string {
	return c.convertVariable(url)
}

// convertVariable converts Insomnia variable syntax to hitspec syntax.
// Insomnia uses {{ _.variableName }} or {{ variableName }}
func (c *Converter) convertVariable(s string) string {
	// Convert {{ _.variableName }} to {{variableName}}
	s = regexp.MustCompile(`\{\{\s*_\.(\w+)\s*\}\}`).ReplaceAllString(s, "{{$1}}")
	// Convert {{ variableName }} to {{variableName}} (normalize spaces)
	s = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`).ReplaceAllString(s, "{{$1}}")
	return s
}

// sanitizeName sanitizes a name for use as an identifier.
func sanitizeName(name string) string {
	// Replace non-alphanumeric characters with underscores
	result := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(name, "_")

	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")

	// Collapse multiple underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	// Convert to camelCase
	return strings.ToLower(result)
}
