package parser

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Parser is a recursive descent parser for hitspec test files.
type Parser struct {
	lexer    *Lexer
	curToken Token
	file     string
	warnings []Warning
}

// WarningOutput is where ParseFile prints the warnings of the files it parses
// ("warning: file:line:col: message"). Set it to nil to silence them; use Parse
// and File.Warnings to handle them yourself.
var WarningOutput io.Writer = os.Stderr

// warn records a suspicious-but-accepted construct at the given token.
func (p *Parser) warn(tok Token, format string, args ...any) {
	p.warnAt(tok.Line, tok.Column, format, args...)
}

func (p *Parser) warnAt(line, col int, format string, args ...any) {
	p.warnings = append(p.warnings, Warning{
		File:    p.file,
		Line:    line,
		Column:  col,
		Message: fmt.Sprintf(format, args...),
	})
}

// NewParser creates a new Parser from the given input string.
func NewParser(input string) *Parser {
	p := &Parser{
		lexer: NewLexer(input),
	}
	p.nextToken()
	return p
}

// ParseFile reads and parses a hitspec file from the filesystem.
func ParseFile(path string) (*File, error) {
	f, err := ParseFileQuiet(path)
	if err == nil && WarningOutput != nil {
		for _, w := range f.Warnings {
			fmt.Fprintf(WarningOutput, "warning: %s\n", w)
		}
	}
	return f, err
}

// ParseFileQuiet is ParseFile without printing the warnings; they are in
// File.Warnings.
func ParseFileQuiet(path string) (*File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(content), path)
}

// Parse parses the given input string as a hitspec file with the given filename for error reporting.
func Parse(input, filename string) (*File, error) {
	p := NewParser(input)
	p.file = filename
	f, err := p.parseFile()
	if f != nil {
		sort.SliceStable(p.warnings, func(i, j int) bool {
			if p.warnings[i].Line != p.warnings[j].Line {
				return p.warnings[i].Line < p.warnings[j].Line
			}
			return p.warnings[i].Column < p.warnings[j].Column
		})
		f.Warnings = p.warnings
	}
	return f, err
}

// getSnippet returns the current source line for error context.
func (p *Parser) getSnippet() string {
	return p.lexer.GetCurrentLine()
}

func (p *Parser) nextToken() {
	p.curToken = p.lexer.NextToken()
	for p.curToken.Type == TokenWhitespace || p.curToken.Type == TokenComment {
		p.curToken = p.lexer.NextToken()
	}
}

func (p *Parser) nextTokenRaw() {
	p.curToken = p.lexer.NextToken()
}

func (p *Parser) skipNewlines() {
	for p.curToken.Type == TokenNewline {
		p.nextToken()
	}
}

// parseFile parses the top-level file structure: variables and requests in any
// order. Variable definitions (@var = value) are allowed at the top of the file
// AND between requests; a mid-file variable used to be swallowed into the
// previous request's body.
func (p *Parser) parseFile() (*File, error) {
	file := &File{Path: p.file}
	p.skipNewlines()

	for p.curToken.Type == TokenVariable {
		file.Variables = append(file.Variables, p.parseVariable())
		p.nextToken()
		p.skipNewlines()
	}

	for p.curToken.Type != TokenEOF {
		switch p.curToken.Type {
		case TokenRequestSeparator, TokenMethod, TokenAnnotation:
			// A request may start with its # @directives and no ### line; they
			// used to be skipped here, silently dropping @name, @tags, @auth...
			req, err := p.parseRequest()
			if err != nil {
				return nil, err
			}
			if req != nil {
				file.Requests = append(file.Requests, req)
			}
		case TokenVariable:
			// A variable definition between requests is a file-level variable.
			file.Variables = append(file.Variables, p.parseVariable())
			p.nextToken()
		case TokenAssertionStart, TokenCaptureStart, TokenDBStart, TokenShellStart, TokenMockStart:
			return nil, p.orphanBlockError()
		default:
			p.nextToken()
		}
		p.skipNewlines()
	}

	return file, nil
}

// parseVariable reads a @var = value definition from the current TokenVariable.
func (p *Parser) parseVariable() *Variable {
	return &Variable{
		Name:  p.curToken.Value,
		Value: p.curToken.Literal.(string),
		Line:  p.curToken.Line,
	}
}

// parseRequest parses a single HTTP request including its separator, annotations,
// method, URL, headers, body, assertions, and capture blocks.
func (p *Parser) parseRequest() (*Request, error) {
	req := &Request{
		Metadata: &RequestMetadata{},
		Line:     p.curToken.Line,
	}

	hadSeparator := false
	if p.curToken.Type == TokenRequestSeparator {
		hadSeparator = true
		req.Name = p.curToken.Value
		p.nextToken()
		p.skipNewlines()
	}

	firstDirective := p.curToken
	for p.curToken.Type == TokenAnnotation {
		if err := p.parseAnnotation(req); err != nil {
			return nil, err
		}
		p.nextToken()
		p.skipNewlines()
	}

	// # @directives at the top of a file (before the first ### or variable) that
	// do not lead into a request apply to nothing: say so and move on.
	if !hadSeparator && p.curToken.Type != TokenMethod &&
		(p.curToken.Type == TokenRequestSeparator || p.curToken.Type == TokenVariable || p.curToken.Type == TokenEOF) {
		p.warn(firstDirective, "directive @%s is not followed by a request, so it applies to nothing", firstDirective.Value)
		return nil, nil
	}

	if p.curToken.Type != TokenMethod {
		return nil, &ParseError{
			File:    p.file,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected HTTP method, got " + p.curToken.Value,
			Snippet: p.getSnippet(),
		}
	}
	req.Method = p.curToken.Value

	// The request line, query-parameter lines and headers are read as raw
	// lines, not tokens: their values must reach the runner verbatim (quotes,
	// '#', apostrophes, spaces inside {{ }}), which the generic tokenizer
	// (built for assertions) cannot guarantee.
	line := p.lexer.takeLineFrom(p.curToken.Offset, p.curToken.Line)
	req.URL, req.HTTPVersion = splitRequestLine(line)
	p.parseRequestHead(req)

	// Resync the token stream at the first line after the headers, skipping the
	// blank line(s) that separate them from the body.
	p.nextToken()
	p.skipNewlines()

	if p.curToken.Type != TokenAssertionStart &&
		p.curToken.Type != TokenCaptureStart &&
		p.curToken.Type != TokenDBStart &&
		p.curToken.Type != TokenShellStart &&
		p.curToken.Type != TokenMockStart &&
		p.curToken.Type != TokenRequestSeparator &&
		p.curToken.Type != TokenEOF {
		body, err := p.parseBody()
		if err != nil {
			return nil, err
		}
		req.Body = body
	}

	// Parse blocks in any order (assertions, db, shell, capture)
	for {
		p.skipNewlines()

		switch {
		case p.curToken.Type == TokenAssertionStart && p.curToken.Value == "":
			assertions, err := p.parseAssertions()
			if err != nil {
				return nil, err
			}
			req.Assertions = append(req.Assertions, assertions...)

		case p.curToken.Type == TokenDBStart:
			dbAssertions, err := p.parseDBBlock()
			if err != nil {
				return nil, err
			}
			req.DBAssertions = append(req.DBAssertions, dbAssertions...)

		case p.curToken.Type == TokenShellStart:
			shellCommands, err := p.parseShellBlock()
			if err != nil {
				return nil, err
			}
			req.ShellCommands = append(req.ShellCommands, shellCommands...)

		case p.curToken.Type == TokenCaptureStart:
			captures, err := p.parseCaptures()
			if err != nil {
				return nil, err
			}
			req.Captures = append(req.Captures, captures...)

		case p.curToken.Type == TokenMockStart:
			mockBody, err := p.parseMockBlock()
			if err != nil {
				return nil, err
			}
			req.MockBody = mockBody

		case p.curToken.Type == TokenAssertionStart:
			// ">>>" followed by a name that is not a block type: a typo such as
			// ">>>captur" used to skip the whole block silently.
			return nil, p.unknownBlockError()

		default:
			return req, nil
		}
	}
}

// blockNames are the block types that can follow ">>>".
var blockNames = []string{"capture", "db", "shell", "mock", "multipart", "graphql", "variables"}

func (p *Parser) unknownBlockError() error {
	name := p.curToken.Value
	msg := fmt.Sprintf("unknown block type >>>%s", name)
	first, args, _ := strings.Cut(name, " ")
	if args != "" && closest(first, blockNames, true) == first {
		msg = fmt.Sprintf("the >>>%s block takes no arguments after it (got %q)", first, strings.TrimSpace(args))
		if first == "db" {
			msg += "; set the connection with a \"# @db <connection>\" directive above the request"
		}
	} else if hint := closest(name, blockNames, true); hint != "" {
		msg += fmt.Sprintf(" (did you mean >>>%s?)", hint)
	} else {
		msg += " (blocks are >>> for assertions, >>>capture, >>>db, >>>shell, >>>mock, >>>multipart, >>>graphql, >>>variables)"
	}
	return &ParseError{
		File:    p.file,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: msg,
		Snippet: p.getSnippet(),
	}
}

// orphanBlockError reports a >>> block that is not part of any request: its
// assertions or captures would never run.
func (p *Parser) orphanBlockError() error {
	if p.curToken.Type == TokenAssertionStart && p.curToken.Value != "" {
		return p.unknownBlockError()
	}
	name := ">>>" + p.curToken.Value
	return &ParseError{
		File:    p.file,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: fmt.Sprintf("%s block is not inside a request (it must follow a request line such as GET /path)", name),
		Snippet: p.getSnippet(),
	}
}

// headerLineRe matches a "Key: Value" header line (the key is an identifier
// that may contain digits, '-', '.' and '_').
var headerLineRe = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_.\-]*[ \t]*:`)

// parseRequestHead reads everything between the request line and the body:
// "? key = value" query parameters and "Key: Value" headers. It reads raw lines
// from the lexer. A blank line, a separator, a block marker or any line that is
// not a header ends the head; comment lines in between are skipped, so headers
// after a comment are still headers. The terminating line is left unconsumed.
func (p *Parser) parseRequestHead(req *Request) {
	l := p.lexer
	seenHeader := false
	for !l.atEOF() {
		text := strings.TrimSpace(l.peekLine())
		switch {
		case text == "", strings.HasPrefix(text, "###"), strings.HasPrefix(text, ">>>"):
			return
		case strings.HasPrefix(text, "#"), strings.HasPrefix(text, "//"):
			// Plain comment inside the head. Even "# @name x" is only a comment
			// here: directives belong before the request line.
			l.skipLine()
		case !seenHeader && (text[0] == '?' || text[0] == '&'):
			req.QueryParams = append(req.QueryParams, parseQueryParamLine(text, l.line))
			l.skipLine()
			// Blank lines between query parameter lines are allowed.
			for !l.atEOF() && strings.TrimSpace(l.peekLine()) == "" {
				l.skipLine()
			}
		case headerLineRe.MatchString(text):
			key, value, _ := strings.Cut(text, ":")
			req.Headers = append(req.Headers, &Header{
				Key:   strings.TrimSpace(key),
				Value: strings.TrimSpace(value),
				Line:  l.line,
			})
			seenHeader = true
			l.skipLine()
		default:
			return
		}
	}
}

// parseQueryParamLine parses a "? key = value" (or "& key = value") line. The
// value is everything after the first '=', trimmed, otherwise untouched.
func parseQueryParamLine(text string, line int) *QueryParam {
	rest := strings.TrimSpace(text[1:])
	key, value, found := strings.Cut(rest, "=")
	if !found {
		key, value, _ = strings.Cut(rest, " ")
	}
	return &QueryParam{
		Key:   strings.TrimSpace(key),
		Value: strings.TrimSpace(value),
		Line:  line,
	}
}

// httpVersionRe matches the optional trailing protocol token of a request line.
var httpVersionRe = regexp.MustCompile(`(?i)^HTTP/\d+(\.\d+)?$`)

// splitRequestLine splits "METHOD URL [HTTP/x.y]". The method is the first
// token, an optional final HTTP/<version> token is the version, and the URL is
// everything in between. Whitespace inside {{ ... }} never splits anything, so
// "{{ base }}/q?x={{$random(1, 100)}}" is one URL. A whitespace-separated token
// starting with '#' begins a trailing comment.
func splitRequestLine(line string) (url, version string) {
	fields := fieldsOutsideBraces(line)
	for i := 2; i < len(fields); i++ {
		if strings.HasPrefix(fields[i].text, "#") {
			fields = fields[:i]
			break
		}
	}
	if len(fields) < 2 {
		return "", ""
	}
	if len(fields) >= 3 && httpVersionRe.MatchString(fields[len(fields)-1].text) {
		version = fields[len(fields)-1].text
		fields = fields[:len(fields)-1]
	}
	return line[fields[1].start:fields[len(fields)-1].end], version
}

// field is a whitespace-delimited piece of a string with its byte span.
type field struct {
	text       string
	start, end int
}

// fieldsOutsideBraces splits s on whitespace that is not inside {{ ... }}.
func fieldsOutsideBraces(s string) []field {
	return splitOutsideBraces(s, func(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }, true)
}

// splitOutsideBraces splits s at every byte for which sep is true, except
// inside {{ ... }}. With dropEmpty, empty pieces are omitted; without it they
// are kept (so "a,,b" splits into three).
func splitOutsideBraces(s string, sep func(byte) bool, dropEmpty bool) []field {
	var out []field
	start := 0
	depth := 0
	flush := func(end int) {
		if end > start || !dropEmpty {
			out = append(out, field{text: s[start:end], start: start, end: end})
		}
	}
	for i := 0; i < len(s); i++ {
		switch {
		case strings.HasPrefix(s[i:], "{{"):
			depth++
			i++
		case depth > 0 && strings.HasPrefix(s[i:], "}}"):
			depth--
			i++
		case depth == 0 && sep(s[i]):
			flush(i)
			start = i + 1
		}
	}
	flush(len(s))
	return out
}
