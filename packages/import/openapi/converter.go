// Package openapi provides functionality to convert OpenAPI/Swagger specifications to hitspec format.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/result"
	"github.com/getkin/kin-openapi/openapi3"
)

// Converter converts OpenAPI specs to hitspec format
type Converter struct {
	baseURL       string
	includeTags   []string
	excludeTags   []string
	includeOnly   []string // specific operation IDs
	generateTests bool
	timeout       time.Duration   // budget for loading a spec (and its $refs) from a URL
	ctx           context.Context // parent of the load deadline; nil means Background
}

// DefaultLoadTimeout bounds how long loading a spec from a URL may take.
const DefaultLoadTimeout = 30 * time.Second

// maxSpecBytes caps a downloaded spec (or $ref) so a hostile or broken server
// cannot make the importer buffer without limit.
const maxSpecBytes = 64 << 20

// Option is a functional option for Converter
type Option func(*Converter)

// WithBaseURL sets a custom base URL, overriding the one from spec
func WithBaseURL(url string) Option {
	return func(c *Converter) {
		c.baseURL = url
	}
}

// WithTimeout sets the time allowed for downloading a spec from a URL,
// $ref documents included (default DefaultLoadTimeout; 0 keeps the default).
func WithTimeout(d time.Duration) Option {
	return func(c *Converter) {
		c.timeout = d
	}
}

// WithContext makes loading a spec from a URL cancellable from ctx.
func WithContext(ctx context.Context) Option {
	return func(c *Converter) {
		c.ctx = ctx
	}
}

// WithTags filters operations by tags
func WithTags(tags []string) Option {
	return func(c *Converter) {
		c.includeTags = tags
	}
}

// WithExcludeTags excludes operations with these tags
func WithExcludeTags(tags []string) Option {
	return func(c *Converter) {
		c.excludeTags = tags
	}
}

// WithOperations filters to specific operation IDs
func WithOperations(ops []string) Option {
	return func(c *Converter) {
		c.includeOnly = ops
	}
}

// WithTests generates test assertions from response schemas
func WithTests(generate bool) Option {
	return func(c *Converter) {
		c.generateTests = generate
	}
}

// NewConverter creates a new OpenAPI converter
func NewConverter(opts ...Option) *Converter {
	c := &Converter{
		generateTests: true,
		timeout:       DefaultLoadTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.timeout <= 0 {
		c.timeout = DefaultLoadTimeout
	}
	return c
}

// loadDoc loads a spec from a file path or URL.
func (c *Converter) loadDoc(path string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var doc *openapi3.T
	var err error

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		doc, err = c.loadFromURL(loader, path)
	} else {
		doc, err = loader.LoadFromFile(path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}
	return doc, nil
}

// newSpecClient is the HTTP client used to download specs: proxy settings from
// the environment, bounded dial/TLS/header waits.
func newSpecClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: timeout,
			ForceAttemptHTTP2:     true,
		},
	}
}

// loadFromURL downloads the spec and the documents it references over HTTP(S)
// under one deadline. The client honors HTTP_PROXY/HTTPS_PROXY/NO_PROXY, and
// only http(s) references are followed (a remote spec cannot make the importer
// read local files).
func (c *Converter) loadFromURL(loader *openapi3.Loader, specURL string) (*openapi3.T, error) {
	u, err := url.Parse(specURL)
	if err != nil {
		return nil, fmt.Errorf("invalid spec URL: %w", err)
	}

	parent := c.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()

	client := newSpecClient(c.timeout)
	defer client.CloseIdleConnections()

	loader.Context = ctx
	loader.ReadFromURIFunc = openapi3.URIMapCache(func(_ *openapi3.Loader, loc *url.URL) ([]byte, error) {
		if loc.Scheme != "http" && loc.Scheme != "https" {
			return nil, openapi3.ErrURINotSupported
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("fetching %s: timed out after %s", loc, c.timeout)
			}
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch spec %s: %s", loc, resp.Status)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("fetching %s: timed out after %s", loc, c.timeout)
			}
			return nil, err
		}
		if len(data) > maxSpecBytes {
			return nil, fmt.Errorf("spec %s is larger than %d MiB", loc, maxSpecBytes>>20)
		}
		return data, nil
	})

	return loader.LoadFromURI(u)
}

// ConvertFile converts an OpenAPI file to hitspec format
func (c *Converter) ConvertFile(path string) (string, error) {
	doc, err := c.loadDoc(path)
	if err != nil {
		return "", err
	}
	return c.Convert(doc)
}

// ConvertFileResult converts an OpenAPI file and reports what could not be
// carried over.
func (c *Converter) ConvertFileResult(path string) (*result.Result, error) {
	doc, err := c.loadDoc(path)
	if err != nil {
		return nil, err
	}
	return c.ConvertResult(doc)
}

// File is one generated .http file.
type File struct {
	Name    string // e.g. "users.http"
	Content string
}

// Split is the outcome of converting a spec into one file per tag.
type Split struct {
	Files    []File
	Warnings []string
}

// ConvertFilesResult converts an OpenAPI file into one .http file per tag
// (untagged operations are grouped by their first path segment). Every file
// carries the @baseUrl and the path parameter variables its requests use.
func (c *Converter) ConvertFilesResult(path string) (*Split, error) {
	doc, err := c.loadDoc(path)
	if err != nil {
		return nil, err
	}
	st, entries := c.collect(doc)
	groups := map[string][]*opEntry{}
	var names []string
	for _, e := range entries {
		if _, ok := groups[e.group]; !ok {
			names = append(names, e.group)
		}
		groups[e.group] = append(groups[e.group], e)
	}
	sort.Strings(names)
	out := &Split{Warnings: st.warnings}
	for _, n := range names {
		out.Files = append(out.Files, File{Name: n + ".http", Content: c.render(doc, st, groups[n])})
	}
	return out, nil
}

// Convert converts an OpenAPI document to hitspec format
func (c *Converter) Convert(doc *openapi3.T) (string, error) {
	res, err := c.ConvertResult(doc)
	if err != nil {
		return "", err
	}
	for _, w := range res.Warnings {
		if strings.HasPrefix(w, validationWarning) {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
	}
	return res.Content, nil
}

const validationWarning = "OpenAPI spec validation:"

// ConvertResult converts an OpenAPI document and reports what could not be
// carried over.
func (c *Converter) ConvertResult(doc *openapi3.T) (*result.Result, error) {
	st, entries := c.collect(doc)
	return &result.Result{Content: c.render(doc, st, entries), Warnings: st.warnings}, nil
}

// state is the per-conversion bookkeeping for path parameter variables.
type state struct {
	baseURL  string
	defaults map[string]string // variable name -> default value
	order    []string          // variable names in first-use order
	warnings []string
}

func (st *state) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, w := range st.warnings {
		if w == msg {
			return
		}
	}
	st.warnings = append(st.warnings, msg)
}

// variable registers a path parameter variable and returns the name to use.
// Two operations that share a parameter name but need different defaults (an
// integer {id} and a UUID {id}) get distinct variables.
func (st *state) variable(name, def, opName string) string {
	if cur, ok := st.defaults[name]; !ok {
		st.defaults[name] = def
		st.order = append(st.order, name)
		return name
	} else if cur == def {
		return name
	}
	alt := sanitizeName(opName) + "_" + name
	for i := 2; ; i++ {
		cur, ok := st.defaults[alt]
		if !ok {
			st.defaults[alt] = def
			st.order = append(st.order, alt)
			return alt
		}
		if cur == def {
			return alt
		}
		alt = fmt.Sprintf("%s_%s_%d", sanitizeName(opName), name, i)
	}
}

// opEntry is one converted operation.
type opEntry struct {
	text  string
	vars  []string // variable names the operation uses
	group string   // output file stem
}

// collect converts every included operation, in path then method order.
func (c *Converter) collect(doc *openapi3.T) (*state, []*opEntry) {
	st := &state{defaults: map[string]string{}}
	if err := doc.Validate(context.Background()); err != nil {
		// Some specs have minor validation issues; convert anyway.
		st.warn("%s %v", validationWarning, err)
	}

	st.baseURL = c.baseURL
	if st.baseURL == "" {
		st.baseURL = c.getBaseURL(doc)
		if len(doc.Servers) == 0 || doc.Servers[0].URL == "" {
			st.warn("the spec lists no servers; @baseUrl defaults to %s", st.baseURL)
		}
	}

	// Get sorted paths for consistent output
	var paths []string
	if doc.Paths != nil {
		for path := range doc.Paths.Map() {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	var entries []*opEntry
	securityUsed := false
	for _, path := range paths {
		pathItem := doc.Paths.Map()[path]
		if pathItem == nil {
			continue
		}

		operations := []struct {
			method string
			op     *openapi3.Operation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"PATCH", pathItem.Patch},
			{"DELETE", pathItem.Delete},
			{"HEAD", pathItem.Head},
			{"OPTIONS", pathItem.Options},
		}

		for _, op := range operations {
			if op.op == nil || !c.shouldInclude(op.op) {
				continue
			}
			text, vars := c.convertOperation(st, path, op.method, op.op, pathItem.Parameters)
			entries = append(entries, &opEntry{text: text, vars: vars, group: groupFor(path, op.op)})
			if (op.op.Security != nil && len(*op.op.Security) > 0) || len(doc.Security) > 0 {
				securityUsed = true
			}
		}
	}
	if securityUsed {
		st.warn("the spec declares security requirements; authentication was not imported (add # @auth to the requests that need it)")
	}
	return st, entries
}

// groupFor picks the output file stem for an operation: its first tag, else
// the first non-parameter path segment.
func groupFor(path string, op *openapi3.Operation) string {
	if len(op.Tags) > 0 {
		if n := fileStem(op.Tags[0]); n != "" {
			return n
		}
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || strings.HasPrefix(seg, "{") {
			continue
		}
		if n := fileStem(seg); n != "" {
			return n
		}
	}
	return "root"
}

var stemBad = regexp.MustCompile(`[^a-z0-9]+`)

func fileStem(s string) string {
	return strings.Trim(stemBad.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// render writes the file header, variables and the given operations.
func (c *Converter) render(doc *openapi3.T, st *state, entries []*opEntry) string {
	var sb strings.Builder

	// Write header comment
	sb.WriteString("# Generated from OpenAPI spec")
	if doc.Info != nil && doc.Info.Title != "" {
		sb.WriteString(": ")
		sb.WriteString(strings.ReplaceAll(doc.Info.Title, "\n", " "))
	}
	sb.WriteString("\n")
	if doc.Info != nil && doc.Info.Version != "" {
		sb.WriteString("# Version: ")
		sb.WriteString(doc.Info.Version)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Base URL and the path parameter variables these operations use.
	if st.baseURL != "" {
		sb.WriteString("@baseUrl = ")
		sb.WriteString(st.baseURL)
		sb.WriteString("\n")
	}
	used := map[string]bool{}
	for _, e := range entries {
		for _, v := range e.vars {
			used[v] = true
		}
	}
	wroteVar := false
	for _, v := range st.order {
		if used[v] {
			fmt.Fprintf(&sb, "@%s = %s\n", v, st.defaults[v])
			wroteVar = true
		}
	}
	if st.baseURL != "" || wroteVar {
		sb.WriteString("\n")
	}

	for _, e := range entries {
		sb.WriteString(e.text)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (c *Converter) getBaseURL(doc *openapi3.T) string {
	if len(doc.Servers) > 0 && doc.Servers[0].URL != "" {
		return doc.Servers[0].URL
	}
	return "http://localhost:3000"
}

func (c *Converter) shouldInclude(op *openapi3.Operation) bool {
	// Check operation ID filter
	if len(c.includeOnly) > 0 {
		found := false
		for _, id := range c.includeOnly {
			if op.OperationID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tag filters
	if len(c.includeTags) > 0 {
		found := false
		for _, tag := range op.Tags {
			for _, includeTag := range c.includeTags {
				if tag == includeTag {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	// Check exclude tags
	if len(c.excludeTags) > 0 {
		for _, tag := range op.Tags {
			for _, excludeTag := range c.excludeTags {
				if tag == excludeTag {
					return false
				}
			}
		}
	}

	return true
}

// concatParams returns pathParams followed by opParams without aliasing
// either slice.
func concatParams(pathParams, opParams openapi3.Parameters) openapi3.Parameters {
	all := make(openapi3.Parameters, 0, len(pathParams)+len(opParams))
	all = append(all, pathParams...)
	return append(all, opParams...)
}

func (c *Converter) convertOperation(st *state, path, method string, op *openapi3.Operation, pathParams openapi3.Parameters) (string, []string) {
	var sb strings.Builder

	// Request separator with name
	name := op.OperationID
	if name == "" {
		name = strings.ToLower(method) + strings.ReplaceAll(toTitle(path), "/", "")
	}
	sb.WriteString("### ")
	if op.Summary != "" {
		sb.WriteString(strings.ReplaceAll(op.Summary, "\n", " "))
	} else {
		sb.WriteString(name)
	}
	sb.WriteString("\n")

	// Annotations
	sb.WriteString("# @name ")
	sb.WriteString(sanitizeName(name))
	sb.WriteString("\n")

	if len(op.Tags) > 0 {
		sb.WriteString("# @tags ")
		sb.WriteString(strings.Join(op.Tags, ","))
		sb.WriteString("\n")
	}

	if op.Description != "" {
		sb.WriteString("# @description ")
		// Single line description
		desc := strings.ReplaceAll(op.Description, "\n", " ")
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		sb.WriteString(desc)
		sb.WriteString("\n")
	}

	// Notes are collected while building the rest and written before the
	// request line.
	var notes []string
	note := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		notes = append(notes, msg)
		st.warn("%s %s: %s", method, path, msg)
	}

	allParams := concatParams(pathParams, op.Parameters)

	// Convert path parameters
	convertedPath, vars := c.convertPathParams(st, path, name, allParams)

	var req strings.Builder
	req.WriteString(method)
	req.WriteString(" {{baseUrl}}")
	req.WriteString(convertedPath)
	req.WriteString("\n")

	// Query parameters
	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In == "query" {
			req.WriteString("  ? ")
			req.WriteString(param.Name)
			req.WriteString(" = ")
			req.WriteString(c.getParamExample(param, false))
			req.WriteString("\n")
		}
	}

	// Headers
	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In == "header" {
			req.WriteString(param.Name)
			req.WriteString(": ")
			req.WriteString(c.getParamExample(param, false))
			req.WriteString("\n")
		}
	}

	// Request body: content types are visited in sorted order so output is
	// deterministic when a spec offers several.
	body := ""
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		var ct string
		ct, body = c.generateRequestBody(op.RequestBody.Value, note)
		switch ct {
		case "json":
			req.WriteString("Content-Type: application/json\n")
		case "form":
			req.WriteString("Content-Type: application/x-www-form-urlencoded\n")
		}
		if body != "" {
			req.WriteString("\n")
			req.WriteString(body)
			req.WriteString("\n")
		}
	}

	for _, n := range notes {
		sb.WriteString("# TODO: ")
		sb.WriteString(strings.ReplaceAll(n, "\n", " "))
		sb.WriteString("\n")
	}
	sb.WriteString(req.String())

	// Assertions
	if c.generateTests {
		assertions := c.generateAssertions(op)
		if assertions != "" {
			sb.WriteString("\n")
			sb.WriteString(assertions)
		}
	}

	return sb.String(), vars
}

var pathTemplateParam = regexp.MustCompile(`\{([^{}/]+)\}`)

// convertPathParams rewrites {id} to {{id}} and registers a variable holding
// the spec's example (or a type-based default) so the request runs as
// generated. It returns the rewritten path and the variables it uses.
func (c *Converter) convertPathParams(st *state, path, opName string, allParams openapi3.Parameters) (string, []string) {
	declared := map[string]*openapi3.Parameter{}
	for _, paramRef := range allParams {
		if paramRef != nil && paramRef.Value != nil && paramRef.Value.In == "path" {
			declared[paramRef.Value.Name] = paramRef.Value
		}
	}
	var vars []string
	out := pathTemplateParam.ReplaceAllStringFunc(path, func(m string) string {
		pname := m[1 : len(m)-1]
		def := "example"
		if p, ok := declared[pname]; ok {
			def = c.getParamExample(p, true)
		} else {
			st.warn("path %s uses {%s} but declares no such parameter; defaulting it to %q", path, pname, def)
		}
		v := st.variable(pname, def, opName)
		vars = append(vars, v)
		return "{{" + v + "}}"
	})
	return out, vars
}

// getParamExample returns a concrete value for a parameter: the spec's
// example, then its default, then its first enum value, then one derived from
// the type. forPath asks for a value that is stable across runs.
func (c *Converter) getParamExample(param *openapi3.Parameter, forPath bool) string {
	if param.Example != nil {
		return result.Scalar(param.Example)
	}
	for _, ex := range sortedExamples(param.Examples) {
		if ex != nil {
			return result.Scalar(ex)
		}
	}

	if param.Schema != nil && param.Schema.Value != nil {
		schema := param.Schema.Value
		if schema.Example != nil {
			return result.Scalar(schema.Example)
		}
		if schema.Default != nil {
			return result.Scalar(schema.Default)
		}
		if len(schema.Enum) > 0 {
			return result.Scalar(schema.Enum[0])
		}

		// Generate based on type
		types := schema.Type.Slice()
		if len(types) > 0 {
			switch types[0] {
			case "integer":
				return "1"
			case "number":
				return "1.0"
			case "boolean":
				return "true"
			case "string":
				switch schema.Format {
				case "date":
					return "2024-01-01"
				case "date-time":
					return "2024-01-01T00:00:00Z"
				case "email":
					return "user@example.com"
				case "uuid":
					if forPath {
						return "123e4567-e89b-12d3-a456-426614174000"
					}
					return "{{$uuid()}}"
				}
				return "example"
			}
		}
	}

	return "example"
}

// sortedExamples returns the values of an examples map in key order.
func sortedExamples(m openapi3.Examples) []any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []any
	for _, k := range keys {
		if m[k] != nil && m[k].Value != nil {
			out = append(out, m[k].Value.Value)
		}
	}
	return out
}

// generateRequestBody returns the body kind ("json", "form", "multipart" or
// "") and its text. Content types are tried in a fixed order of preference.
func (c *Converter) generateRequestBody(reqBody *openapi3.RequestBody, note func(string, ...any)) (string, string) {
	types := make([]string, 0, len(reqBody.Content))
	for ct := range reqBody.Content {
		types = append(types, ct)
	}
	sort.Strings(types)

	// Prefer JSON
	for _, ct := range types {
		mt := reqBody.Content[ct]
		if strings.Contains(ct, "json") && mt.Schema != nil {
			if s := mt.Schema.Value; s != nil && len(s.Type.Slice()) == 0 && (len(s.AllOf)+len(s.OneOf)+len(s.AnyOf) > 0) {
				note("the %s body schema uses allOf/oneOf/anyOf; the generated body is a placeholder", ct)
			}
			return "json", c.generateJSONFromSchema(mt.Schema.Value, 0)
		}
	}

	// Try form data
	for _, ct := range types {
		mt := reqBody.Content[ct]
		if strings.Contains(ct, "x-www-form-urlencoded") && mt.Schema != nil {
			return "form", c.generateFormFromSchema(mt.Schema.Value)
		}
	}

	// Multipart
	for _, ct := range types {
		mt := reqBody.Content[ct]
		if strings.HasPrefix(ct, "multipart/") && mt.Schema != nil {
			return "multipart", c.generateMultipartFromSchema(mt.Schema.Value, note)
		}
	}
	for _, ct := range types {
		if strings.Contains(ct, "form") && reqBody.Content[ct].Schema != nil {
			return "form", c.generateFormFromSchema(reqBody.Content[ct].Schema.Value)
		}
	}

	if len(types) > 0 {
		note("request body content type %s is not supported; the body was left out", strings.Join(types, ", "))
	}
	return "", ""
}

// generateMultipartFromSchema renders a >>>multipart block; binary properties
// become file lines.
func (c *Converter) generateMultipartFromSchema(schema *openapi3.Schema, note func(string, ...any)) string {
	if schema == nil || len(schema.Properties) == 0 {
		return ""
	}
	names := make([]string, 0, len(schema.Properties))
	for n := range schema.Properties {
		names = append(names, n)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString(">>>multipart\n")
	for _, n := range names {
		ref := schema.Properties[n]
		var prop *openapi3.Schema
		if ref != nil {
			prop = ref.Value
		}
		if prop != nil && prop.Format == "binary" {
			fmt.Fprintf(&sb, "file @./%s\n", n)
			note("multipart file field name %q cannot be expressed by hitspec's `file @path` line; point it at a real file and set the name by hand", n)
			continue
		}
		value := "example"
		if prop != nil && prop.Example != nil {
			value = result.Scalar(prop.Example)
		}
		fmt.Fprintf(&sb, "field %s = %s\n", n, value)
	}
	sb.WriteString("<<<")
	return sb.String()
}

func (c *Converter) generateJSONFromSchema(schema *openapi3.Schema, depth int) string {
	if schema == nil || depth > 5 {
		return "{}"
	}

	if len(schema.Type.Slice()) == 0 {
		return "{}"
	}

	switch schema.Type.Slice()[0] {
	case "object":
		var sb strings.Builder
		sb.WriteString("{\n")

		props := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			props = append(props, name)
		}
		sort.Strings(props)

		for i, name := range props {
			propSchema := schema.Properties[name]
			indent := strings.Repeat("  ", depth+1)
			sb.WriteString(indent)
			sb.WriteString("\"")
			sb.WriteString(name)
			sb.WriteString("\": ")

			if propSchema != nil && propSchema.Value != nil {
				sb.WriteString(c.generateJSONValue(propSchema.Value, depth+1))
			} else {
				sb.WriteString("null")
			}

			if i < len(props)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}

		sb.WriteString(strings.Repeat("  ", depth))
		sb.WriteString("}")
		return sb.String()

	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			item := c.generateJSONValue(schema.Items.Value, depth+1)
			return "[" + item + "]"
		}
		return "[]"

	default:
		return c.generateJSONValue(schema, depth)
	}
}

func (c *Converter) generateJSONValue(schema *openapi3.Schema, depth int) string {
	if schema == nil {
		return "null"
	}

	// Use example if available
	if schema.Example != nil {
		data, err := json.Marshal(schema.Example)
		if err == nil {
			return string(data)
		}
	}

	if len(schema.Type.Slice()) == 0 {
		return "null"
	}

	switch schema.Type.Slice()[0] {
	case "string":
		if schema.Format == "date" {
			return "\"2024-01-01\""
		} else if schema.Format == "date-time" {
			return "\"2024-01-01T00:00:00Z\""
		} else if schema.Format == "email" {
			return "\"user@example.com\""
		} else if schema.Format == "uuid" {
			return "\"{{$uuid()}}\""
		}
		if len(schema.Enum) > 0 {
			return fmt.Sprintf("\"%v\"", schema.Enum[0])
		}
		return "\"example\""
	case "integer":
		if schema.Min != nil {
			return fmt.Sprintf("%.0f", *schema.Min)
		}
		return "1"
	case "number":
		if schema.Min != nil {
			return fmt.Sprintf("%v", *schema.Min)
		}
		return "1.0"
	case "boolean":
		return "true"
	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			item := c.generateJSONValue(schema.Items.Value, depth+1)
			return "[" + item + "]"
		}
		return "[]"
	case "object":
		return c.generateJSONFromSchema(schema, depth)
	default:
		return "null"
	}
}

func (c *Converter) generateFormFromSchema(schema *openapi3.Schema) string {
	if schema == nil || len(schema.Properties) == 0 {
		return ""
	}

	var parts []string
	for name, propSchema := range schema.Properties {
		value := "example"
		if propSchema != nil && propSchema.Value != nil {
			if propSchema.Value.Example != nil {
				value = result.Scalar(propSchema.Value.Example)
			}
		}
		parts = append(parts, name+"="+value)
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func (c *Converter) generateAssertions(op *openapi3.Operation) string {
	if op.Responses == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(">>>\n")

	// Find the success response (2xx). Codes are sorted so a spec with several
	// 2xx responses always yields the same assertion.
	codes := make([]string, 0, len(op.Responses.Map()))
	for code := range op.Responses.Map() {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	var successCode string
	var successResp *openapi3.Response
	for _, code := range codes {
		respRef := op.Responses.Map()[code]
		if strings.HasPrefix(code, "2") && respRef != nil && respRef.Value != nil {
			successCode = code
			successResp = respRef.Value
			break
		}
	}

	if successCode != "" {
		if strings.ContainsAny(successCode, "xX") {
			// "2XX" is a range, not a status code.
			sb.WriteString("expect status >= 200\nexpect status < 300\n")
		} else {
			sb.WriteString("expect status == ")
			sb.WriteString(successCode)
			sb.WriteString("\n")
		}

		// Add content-type assertion if JSON
		if successResp != nil {
			cts := make([]string, 0, len(successResp.Content))
			for contentType := range successResp.Content {
				cts = append(cts, contentType)
			}
			sort.Strings(cts)
			for _, contentType := range cts {
				if strings.Contains(contentType, "json") {
					sb.WriteString("expect header Content-Type contains application/json\n")
					break
				}
			}
		}
	} else {
		// Default to 200
		sb.WriteString("expect status == 200\n")
	}

	sb.WriteString("<<<\n")
	return sb.String()
}

func sanitizeName(name string) string {
	// Remove special characters and convert to camelCase
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	// Remove consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")

	return result
}

// toTitle converts a string to title case (simple implementation)
func toTitle(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '/' || r == '-' || r == '_' || r == ' ' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext && r >= 'a' && r <= 'z' {
			result.WriteRune(r - 32) // Convert to uppercase
			capitalizeNext = false
		} else {
			result.WriteRune(r)
			capitalizeNext = false
		}
	}
	return result.String()
}

// ConvertToFile converts and writes to a file
func (c *Converter) ConvertToFile(specPath, outputPath string) error {
	content, err := c.ConvertFile(specPath)
	if err != nil {
		return err
	}

	// Create directory if needed
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return os.WriteFile(outputPath, []byte(content), 0644)
}
