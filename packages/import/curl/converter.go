// Package curl provides curl command to hitspec format conversion.
package curl

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/result"
)

// Converter converts curl commands to hitspec format.
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

// NewConverter creates a new curl converter.
func NewConverter(opts ...Option) *Converter {
	c := &Converter{
		generateAssertions: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Header is one request header in command-line order.
type Header struct {
	Key   string
	Value string
}

// FormPart is one -F/--form entry.
type FormPart struct {
	Name  string
	Value string // text field value
	File  string // file path, for file parts
}

// ParsedCurl represents a parsed curl command.
type ParsedCurl struct {
	Method string
	URL    string
	// Headers maps header name to its last value. HeaderList keeps every header
	// in command-line order and is what ToHitspec writes when it is set.
	Headers         map[string]string
	HeaderList      []Header
	Body            string // inline body (multiple -d joined with &)
	BodyFile        string // -d @file / -T file: body read from this path
	Form            []FormPart
	BasicAuth       string
	DigestAuth      bool
	BearerToken     string
	Insecure        bool
	FollowRedirects bool
	TimeoutMs       int
	Name            string
	// Warnings lists every flag or value that could not be carried over.
	Warnings []string
}

// ConvertCommand converts a single curl command to hitspec format.
func (c *Converter) ConvertCommand(curlCmd string) (string, error) {
	res, err := c.ConvertCommandResult(curlCmd)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// ConvertCommandResult converts a single curl command and reports what was
// dropped or approximated.
func (c *Converter) ConvertCommandResult(curlCmd string) (*result.Result, error) {
	parsed, err := c.Parse(curlCmd)
	if err != nil {
		return nil, err
	}
	res := &result.Result{Content: c.ToHitspec(parsed)}
	for _, w := range parsed.Warnings {
		res.Warn("%s", w)
	}
	return res, nil
}

// ConvertFile converts a file containing curl commands to hitspec format.
func (c *Converter) ConvertFile(path string) (string, error) {
	res, err := c.ConvertFileResult(path)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// ConvertFileResult converts a file of curl commands and reports what was
// dropped or approximated.
func (c *Converter) ConvertFileResult(path string) (*result.Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var commands []string
	var currentCmd strings.Builder
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || (currentCmd.Len() == 0 && strings.HasPrefix(line, "#")) {
			continue
		}

		// Handle line continuations
		if strings.HasSuffix(line, "\\") {
			currentCmd.WriteString(strings.TrimSuffix(line, "\\"))
			currentCmd.WriteString(" ")
			continue
		}

		currentCmd.WriteString(line)
		commands = append(commands, currentCmd.String())
		currentCmd.Reset()
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Handle any remaining command
	if currentCmd.Len() > 0 {
		commands = append(commands, currentCmd.String())
	}

	res := &result.Result{}
	var sb strings.Builder
	sb.WriteString("# Generated from curl commands\n\n")

	used := map[string]int{}
	for i, cmd := range commands {
		parsed, err := c.Parse(cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to convert command %d: %w", i+1, err)
		}
		// Two commands to the same endpoint would otherwise share an @name.
		used[parsed.Name]++
		if n := used[parsed.Name]; n > 1 {
			parsed.Name = fmt.Sprintf("%s_%d", parsed.Name, n)
		}
		for _, w := range parsed.Warnings {
			res.Warn("command %d: %s", i+1, w)
		}
		sb.WriteString(c.ToHitspec(parsed))
		sb.WriteString("\n")
	}

	res.Content = sb.String()
	return res, nil
}

// flagKind classifies a curl option.
type flagKind int

const (
	flagUnknown flagKind = iota
	flagBool             // takes no value, no effect on the request
	flagValue            // takes a value, handled or ignored with a warning
)

// shortValueFlags are the single-letter options that take an argument.
const shortValueFlags = "XHduAebFmowxcDTEtQrCPKyYzZ"

// ignoredBoolFlags change how curl prints or transfers but not what request
// it sends, so dropping them loses nothing.
var ignoredBoolFlags = map[string]bool{
	"-s": true, "--silent": true, "-S": true, "--show-error": true,
	"-v": true, "--verbose": true, "-i": true, "--include": true,
	"-f": true, "--fail": true, "--fail-with-body": true,
	"-N": true, "--no-buffer": true, "-g": true, "--globoff": true,
	"-#": true, "--progress-bar": true, "--compressed": true,
	"--http1.0": true, "--http1.1": true, "--http2": true, "--http2-prior-knowledge": true,
	"-4": true, "--ipv4": true, "-6": true, "--ipv6": true,
	"--tcp-nodelay": true, "--no-keepalive": true, "--path-as-is": true,
	"-q": true, "--disable": true, "--basic": true, "-O": true, "--remote-name": true,
	"-J": true, "--remote-header-name": true, "--tr-encoding": true, "--no-progress-meter": true,
}

// ignoredValueFlags take an argument that does not affect the request body,
// headers or URL that hitspec can express; they are reported, not dropped
// silently.
var ignoredValueFlags = map[string]string{
	"-o": "output file", "--output": "output file",
	"-w": "write-out format", "--write-out": "write-out format",
	"-D": "dump-header file", "--dump-header": "dump-header file",
	"-c": "cookie jar", "--cookie-jar": "cookie jar",
	"--connect-timeout": "connect timeout",
	"-x":                "proxy", "--proxy": "proxy",
	"-E": "client certificate", "--cert": "client certificate", "--key": "client key",
	"--cacert": "CA certificate", "--capath": "CA directory",
	"--retry": "retry count", "--retry-delay": "retry delay", "--retry-max-time": "retry max time",
	"-r": "byte range", "--range": "byte range",
	"--resolve": "DNS override", "--interface": "network interface", "--limit-rate": "rate limit",
	"--max-redirs": "redirect limit", "--proto": "protocol list", "--proto-redir": "protocol list",
	"-K": "config file", "--config": "config file",
	"-y": "speed time", "-Y": "speed limit", "-z": "time condition", "-C": "resume offset",
	"-t": "telnet option", "-Q": "quote command", "-P": "ftp port",
}

// Parse parses a curl command string into a ParsedCurl struct.
func (c *Converter) Parse(curlCmd string) (*ParsedCurl, error) {
	parsed := &ParsedCurl{
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Normalize the command: join backslash-newline continuations.
	curlCmd = strings.TrimSpace(curlCmd)
	curlCmd = strings.NewReplacer("\\\r\n", " ", "\\\n", " ").Replace(curlCmd)

	tokens := tokenize(curlCmd)
	if len(tokens) > 0 && tokens[0] == "curl" {
		tokens = tokens[1:]
	} else if len(tokens) == 0 {
		return nil, fmt.Errorf("no URL specified")
	}

	var (
		methodSet bool
		getMode   bool // -G: send data as a query string
		headMode  bool
		dataParts []string
		fileParts []string
		urls      []string
		jsonMode  bool
	)
	warn := func(format string, args ...any) {
		parsed.Warnings = append(parsed.Warnings, fmt.Sprintf(format, args...))
	}
	addHeader := func(k, v string) {
		parsed.Headers[k] = v
		parsed.HeaderList = append(parsed.HeaderList, Header{Key: k, Value: v})
	}
	setHeaderDefault := func(k, v string) {
		for _, h := range parsed.HeaderList {
			if strings.EqualFold(h.Key, k) {
				return
			}
		}
		addHeader(k, v)
	}

	// takeValue resolves the argument of an option: the attached form
	// (-XPOST, --request=POST) or the next token.
	i := 0
	for i < len(tokens) {
		token := tokens[i]
		i++

		if token == "--" {
			// Everything after -- is a URL.
			urls = append(urls, tokens[i:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			urls = append(urls, token)
			continue
		}

		name, attached, hasAttached := token, "", false
		if strings.HasPrefix(token, "--") {
			if eq := strings.Index(token, "="); eq > 0 {
				name, attached, hasAttached = token[:eq], token[eq+1:], true
			}
		} else if len(token) > 2 {
			// Short options: -XPOST, or a cluster such as -sSL.
			first := token[1]
			if strings.IndexByte(shortValueFlags, first) >= 0 {
				name, attached, hasAttached = token[:2], token[2:], true
			} else {
				cluster := token[1:]
				var letters []string
				for _, r := range cluster {
					flag := "-" + string(r)
					if kindOf(flag) != flagBool && !strings.Contains("kLGI", string(r)) {
						letters = nil
						break
					}
					letters = append(letters, flag)
				}
				if letters != nil {
					// Re-dispatch each letter of the cluster as its own token.
					tokens = append(tokens[:i:i], append(letters, tokens[i:]...)...)
					continue
				}
			}
		}

		value := func() (string, bool) {
			if hasAttached {
				return attached, true
			}
			if i < len(tokens) {
				v := tokens[i]
				i++
				return v, true
			}
			return "", false
		}
		need := func() (string, error) {
			v, ok := value()
			if !ok {
				return "", fmt.Errorf("missing value for %s", name)
			}
			return v, nil
		}

		switch name {
		case "-X", "--request":
			v, err := need()
			if err != nil {
				return nil, err
			}
			parsed.Method = strings.ToUpper(v)
			methodSet = true

		case "-H", "--header":
			v, err := need()
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(v, "@") {
				warn("header file %s (%s) was not read; add those headers by hand", v, name)
				break
			}
			parts := strings.SplitN(v, ":", 2)
			switch {
			case len(parts) == 2 && strings.TrimSpace(parts[1]) != "":
				addHeader(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			case len(parts) == 2:
				warn("header %q has an empty value (curl removes a default header); not carried over", strings.TrimSpace(parts[0]))
			case strings.HasSuffix(v, ";"):
				warn("header %q has no value (curl sends it empty); not carried over", strings.TrimSuffix(v, ";"))
			default:
				warn("header %q is not in Name: value form; not carried over", v)
			}

		case "-d", "--data", "--data-ascii", "--data-raw", "--data-binary":
			v, err := need()
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(v, "@") && name != "--data-raw" {
				path := v[1:]
				if path == "-" {
					warn("%s @- reads the body from stdin; supply the body by hand", name)
				} else {
					fileParts = append(fileParts, path)
				}
			} else {
				dataParts = append(dataParts, v)
			}

		case "--json":
			v, err := need()
			if err != nil {
				return nil, err
			}
			jsonMode = true
			if strings.HasPrefix(v, "@") {
				if v == "@-" {
					warn("--json @- reads the body from stdin; supply the body by hand")
				} else {
					fileParts = append(fileParts, v[1:])
				}
			} else {
				dataParts = append(dataParts, v)
			}

		case "--data-urlencode":
			v, err := need()
			if err != nil {
				return nil, err
			}
			enc, ok := urlencodeData(v)
			if !ok {
				warn("--data-urlencode %q reads its value from a file; add that field by hand", v)
				break
			}
			dataParts = append(dataParts, enc)

		case "-F", "--form", "--form-string":
			v, err := need()
			if err != nil {
				return nil, err
			}
			part, ws := parseFormPart(v, name == "--form-string")
			for _, w := range ws {
				warn("%s", w)
			}
			if part != nil {
				parsed.Form = append(parsed.Form, *part)
			}

		case "-u", "--user":
			v, err := need()
			if err != nil {
				return nil, err
			}
			parsed.BasicAuth = v

		case "--digest":
			parsed.DigestAuth = true

		case "--oauth2-bearer":
			v, err := need()
			if err != nil {
				return nil, err
			}
			parsed.BearerToken = v

		case "-k", "--insecure":
			parsed.Insecure = true

		case "-L", "--location":
			parsed.FollowRedirects = true

		case "-G", "--get":
			getMode = true

		case "-I", "--head":
			headMode = true

		case "-A", "--user-agent":
			v, err := need()
			if err != nil {
				return nil, err
			}
			addHeader("User-Agent", v)

		case "-e", "--referer":
			v, err := need()
			if err != nil {
				return nil, err
			}
			addHeader("Referer", v)

		case "-b", "--cookie":
			v, err := need()
			if err != nil {
				return nil, err
			}
			if strings.Contains(v, "=") {
				addHeader("Cookie", v)
			} else {
				warn("cookie file %q was not read; add a Cookie header by hand", v)
			}

		case "-m", "--max-time":
			v, err := need()
			if err != nil {
				return nil, err
			}
			if secs, perr := strconv.ParseFloat(v, 64); perr == nil && secs > 0 {
				parsed.TimeoutMs = int(secs * 1000)
			} else {
				warn("%s %q is not a number of seconds; timeout not carried over", name, v)
			}

		case "-T", "--upload-file":
			v, err := need()
			if err != nil {
				return nil, err
			}
			if v == "-" {
				warn("%s - uploads stdin; supply the body by hand", name)
			} else {
				fileParts = append(fileParts, v)
				if !methodSet {
					parsed.Method = "PUT"
					methodSet = true
				}
			}

		case "--url":
			v, err := need()
			if err != nil {
				return nil, err
			}
			urls = append(urls, v)

		default:
			switch kindOf(name) {
			case flagBool:
				// no effect on the request
			case flagValue:
				if _, ok := value(); !ok {
					return nil, fmt.Errorf("missing value for %s", name)
				}
				warn("%s (%s) has no hitspec equivalent and was ignored", name, ignoredValueFlags[name])
			default:
				// An unknown option may or may not take a value; swallow the
				// next token only when it cannot be a URL or another option.
				if !hasAttached && i < len(tokens) && !strings.HasPrefix(tokens[i], "-") &&
					!strings.Contains(tokens[i], "://") && !strings.HasPrefix(tokens[i], "{{") {
					i++
				}
				warn("unsupported curl option %s was ignored", name)
			}
		}
	}

	// URL
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URL found in curl command")
	}
	parsed.URL = normalizeURL(urls[0])
	if len(urls) > 1 {
		warn("curl command lists %d URLs; only the first (%s) was imported", len(urls), urls[0])
	}
	if strings.ContainsAny(parsed.URL, "[]") && !strings.Contains(parsed.URL, "{{") {
		warn("URL %q contains curl globbing ([] or {}); expand it by hand", parsed.URL)
	}

	// Body
	if jsonMode {
		setHeaderDefault("Content-Type", "application/json")
		setHeaderDefault("Accept", "application/json")
	}
	switch {
	case len(parsed.Form) > 0:
		if len(dataParts)+len(fileParts) > 0 {
			warn("-F cannot be combined with -d/--data; the -d data was ignored")
		}
		if !methodSet && !headMode {
			parsed.Method = "POST"
		}
	case getMode:
		// -G turns the data into a query string.
		if len(fileParts) > 0 {
			warn("-G with -d @file cannot be expressed; the file data was ignored")
		}
		if len(dataParts) > 0 {
			sep := "?"
			if strings.Contains(parsed.URL, "?") {
				sep = "&"
			}
			parsed.URL += sep + strings.Join(dataParts, "&")
		}
	default:
		if len(fileParts) > 0 && len(dataParts) == 0 && len(fileParts) == 1 {
			parsed.BodyFile = fileParts[0]
		} else if len(fileParts) > 0 {
			warn("data files %s cannot be merged with other -d data; keep one source or combine them by hand", strings.Join(fileParts, ", "))
			parsed.Body = strings.Join(dataParts, "&")
		} else {
			parsed.Body = strings.Join(dataParts, "&")
		}
		if (parsed.Body != "" || parsed.BodyFile != "") && !methodSet && !headMode {
			parsed.Method = "POST"
		}
	}
	if headMode && !methodSet {
		parsed.Method = "HEAD"
	}

	if parsed.Insecure {
		warn("-k/--insecure has no per-request equivalent; set validateSSL: false in hitspec.yaml to skip certificate checks")
	}

	// Generate a name from the URL
	parsed.Name = generateName(parsed.URL, parsed.Method)

	return parsed, nil
}

func kindOf(flag string) flagKind {
	if ignoredBoolFlags[flag] {
		return flagBool
	}
	if _, ok := ignoredValueFlags[flag]; ok {
		return flagValue
	}
	return flagUnknown
}

// normalizeURL adds the scheme curl assumes when none is given.
func normalizeURL(u string) string {
	if strings.Contains(u, "://") || strings.HasPrefix(u, "{{") {
		return u
	}
	return "http://" + u
}

var varRef = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// queryEscape percent-encodes s but leaves {{variable}} references intact so
// hitspec can still interpolate them.
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

// urlencodeData applies curl's --data-urlencode forms: content, =content,
// name=content. The name@file and @file forms read a file and are reported as
// not representable (ok == false).
func urlencodeData(v string) (string, bool) {
	if eq := strings.Index(v, "="); eq >= 0 {
		name, content := v[:eq], v[eq+1:]
		if name == "" {
			return queryEscape(content), true
		}
		return name + "=" + queryEscape(content), true
	}
	if strings.Contains(v, "@") {
		return "", false
	}
	return queryEscape(v), true
}

// parseFormPart parses one -F value: name=value, name=@file[;type=..], or
// name=<file. The second result lists constructs hitspec cannot express.
func parseFormPart(v string, literal bool) (*FormPart, []string) {
	eq := strings.Index(v, "=")
	if eq <= 0 {
		return nil, []string{fmt.Sprintf("form field %q is not in name=value form; not carried over", v)}
	}
	name, val := v[:eq], v[eq+1:]
	if literal {
		return &FormPart{Name: name, Value: val}, nil
	}
	switch {
	case strings.HasPrefix(val, "@"):
		path, mods, _ := strings.Cut(val[1:], ";")
		var ws []string
		if strings.Contains(path, ",") {
			ws = append(ws, fmt.Sprintf("form field %q lists several files; only the first was imported", name))
			path = strings.SplitN(path, ",", 2)[0]
		}
		if mods != "" {
			ws = append(ws, fmt.Sprintf("form field %q has modifiers (;%s) that hitspec cannot express; they were dropped", name, mods))
		}
		if !result.CanNameMultipartFile(name) {
			ws = append(ws, fmt.Sprintf("multipart file field name %q cannot be written as `file <name> @path` (it has spaces); the part is sent as field \"file\", set the name by hand", name))
		}
		return &FormPart{Name: name, File: path}, ws
	case strings.HasPrefix(val, "<"):
		return nil, []string{fmt.Sprintf("form field %q reads its value from file %s; add the value by hand", name, strings.TrimPrefix(val, "<"))}
	}
	if value, mods, ok := strings.Cut(val, ";type="); ok {
		return &FormPart{Name: name, Value: value}, []string{fmt.Sprintf("form field %q has a type (;type=%s) that hitspec cannot express; it was dropped", name, mods)}
	}
	return &FormPart{Name: name, Value: val}, nil
}

// ToHitspec converts a ParsedCurl to hitspec format.
func (c *Converter) ToHitspec(parsed *ParsedCurl) string {
	var sb strings.Builder

	// Request separator and name
	sb.WriteString("### ")
	sb.WriteString(parsed.Name)
	sb.WriteString("\n")

	sb.WriteString("# @name ")
	sb.WriteString(sanitizeName(parsed.Name))
	sb.WriteString("\n")

	for _, w := range parsed.Warnings {
		sb.WriteString("# TODO: ")
		sb.WriteString(strings.ReplaceAll(w, "\n", " "))
		sb.WriteString("\n")
	}

	if parsed.TimeoutMs > 0 {
		fmt.Fprintf(&sb, "# @timeout %d\n", parsed.TimeoutMs)
	}

	headers := parsed.HeaderList
	if len(headers) == 0 && len(parsed.Headers) > 0 {
		keys := make([]string, 0, len(parsed.Headers))
		for k := range parsed.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			headers = append(headers, Header{Key: k, Value: parsed.Headers[k]})
		}
	}

	// Auth annotation if present
	switch {
	case parsed.BearerToken != "":
		if strings.Contains(parsed.BearerToken, ",") {
			headers = append(headers, Header{Key: "Authorization", Value: "Bearer " + parsed.BearerToken})
		} else {
			sb.WriteString("# @auth bearer ")
			sb.WriteString(parsed.BearerToken)
			sb.WriteString("\n")
		}
	case parsed.BasicAuth != "":
		user, pass, hasPass := strings.Cut(parsed.BasicAuth, ":")
		kind := "basic"
		if parsed.DigestAuth {
			kind = "digest"
		}
		switch {
		case !hasPass:
			// curl would prompt for the password.
			fmt.Fprintf(&sb, "# @auth %s %s, {{password}}\n", kind, user)
		case pass == "" || strings.Contains(user, ",") || strings.Contains(pass, ","):
			// A comma or empty password cannot be written in the @auth comma
			// syntax; send the header curl would have sent.
			if kind == "digest" {
				sb.WriteString("# TODO: digest credentials contain a comma or are empty; add `# @auth digest user, password` by hand\n")
			} else {
				token := base64.StdEncoding.EncodeToString([]byte(parsed.BasicAuth))
				headers = append(headers, Header{Key: "Authorization", Value: "Basic " + token})
			}
		default:
			fmt.Fprintf(&sb, "# @auth %s %s, %s\n", kind, user, pass)
		}
	}

	// Method and URL
	sb.WriteString(parsed.Method)
	sb.WriteString(" ")
	sb.WriteString(parsed.URL)
	sb.WriteString("\n")

	for _, h := range headers {
		sb.WriteString(h.Key)
		sb.WriteString(": ")
		sb.WriteString(h.Value)
		sb.WriteString("\n")
	}

	// Body
	switch {
	case len(parsed.Form) > 0:
		sb.WriteString("\n>>>multipart\n")
		for _, f := range parsed.Form {
			if f.File != "" {
				sb.WriteString(result.MultipartFileLine(f.Name, relativePath(f.File)))
				sb.WriteString("\n")
			} else {
				fmt.Fprintf(&sb, "field %s = %s\n", f.Name, f.Value)
			}
		}
		sb.WriteString("<<<\n")
	case parsed.BodyFile != "":
		sb.WriteString("\n< ")
		sb.WriteString(relativePath(parsed.BodyFile))
		sb.WriteString("\n")
	case parsed.Body != "":
		sb.WriteString("\n")
		sb.WriteString(parsed.Body)
		sb.WriteString("\n")
	}

	// Assertions
	if c.generateAssertions {
		sb.WriteString("\n>>>\n")
		sb.WriteString("expect status >= 200\n")
		sb.WriteString("expect status < 400\n")
		sb.WriteString("<<<\n")
	}

	return sb.String()
}

// relativePath makes a bare file name explicit (`data.json` -> `./data.json`)
// because hitspec resolves file references relative to the .http file.
func relativePath(p string) string {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "{{") {
		return p
	}
	return "./" + p
}

// tokenize splits a curl command into tokens the way a POSIX shell would:
// single quotes are literal, double quotes allow \" \\ \$ \` escapes, and
// $'...' understands the ANSI-C escapes browsers emit in "Copy as cURL".
func tokenize(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inToken := false
	rs := []rune(cmd)

	flush := func() {
		if inToken {
			tokens = append(tokens, current.String())
			current.Reset()
			inToken = false
		}
	}

	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()

		case r == '\\':
			inToken = true
			if i+1 < len(rs) {
				i++
				current.WriteRune(rs[i])
			}

		case r == '$' && i+1 < len(rs) && rs[i+1] == '\'':
			// ANSI-C quoting.
			inToken = true
			i += 2
			for i < len(rs) && rs[i] != '\'' {
				if rs[i] == '\\' && i+1 < len(rs) {
					i++
					switch rs[i] {
					case 'n':
						current.WriteRune('\n')
					case 't':
						current.WriteRune('\t')
					case 'r':
						current.WriteRune('\r')
					case 'u', 'x':
						n := 4
						if rs[i] == 'x' {
							n = 2
						}
						if i+n < len(rs) {
							if v, ok := hexRune(rs[i+1 : i+1+n]); ok {
								current.WriteRune(v)
								i += n
								break
							}
						}
						current.WriteRune(rs[i])
					default:
						current.WriteRune(rs[i])
					}
				} else {
					current.WriteRune(rs[i])
				}
				i++
			}

		case r == '\'':
			inToken = true
			i++
			for i < len(rs) && rs[i] != '\'' {
				current.WriteRune(rs[i])
				i++
			}

		case r == '"':
			inToken = true
			i++
			for i < len(rs) && rs[i] != '"' {
				if rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune("\"\\$`", rs[i+1]) {
					i++
				}
				current.WriteRune(rs[i])
				i++
			}

		default:
			inToken = true
			current.WriteRune(r)
		}
	}
	flush()

	return tokens
}

// generateName generates a request name from the URL and method.
func generateName(url, method string) string {
	// Extract the path from the URL
	urlPattern := regexp.MustCompile(`https?://[^/]+(/[^?#]*)?`)
	matches := urlPattern.FindStringSubmatch(url)

	path := "/"
	if len(matches) > 1 && matches[1] != "" {
		path = matches[1]
	}

	// Clean up the path for a name
	path = strings.Trim(path, "/")
	if path == "" {
		path = "root"
	}

	// Replace path separators and other characters
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "-", "_")

	return strings.ToLower(method) + "_" + path
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

	return result
}

// hexRune decodes the hex digits of a $'\xHH' or $'\uHHHH' escape.
func hexRune(digits []rune) (rune, bool) {
	var v rune
	for _, c := range digits {
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = c - '0'
		case c >= 'a' && c <= 'f':
			d = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			d = c - 'A' + 10
		default:
			return 0, false
		}
		v = v<<4 | d
	}
	return v, true
}
