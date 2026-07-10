package parser

import (
	"os"
	"strings"
)

// Parser is a recursive descent parser for hitspec test files.
type Parser struct {
	lexer    *Lexer
	curToken Token
	file     string
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
	return p.parseFile()
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

func (p *Parser) skipWhitespace() {
	for p.curToken.Type == TokenWhitespace {
		p.nextTokenRaw()
	}
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
		case TokenRequestSeparator, TokenMethod:
			req, err := p.parseRequest()
			if err != nil {
				return nil, err
			}
			file.Requests = append(file.Requests, req)
		case TokenVariable:
			// A variable definition between requests is a file-level variable.
			file.Variables = append(file.Variables, p.parseVariable())
			p.nextToken()
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

	if p.curToken.Type == TokenRequestSeparator {
		req.Name = p.curToken.Value
		p.nextToken()
		p.skipNewlines()
	}

	for p.curToken.Type == TokenAnnotation {
		if err := p.parseAnnotation(req); err != nil {
			return nil, err
		}
		p.nextToken()
		p.skipNewlines()
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
	p.nextToken()

	if p.curToken.Type == TokenWhitespace {
		p.nextToken()
	}

	url := p.parseURL()
	req.URL = url

	// Advance past the URL line's terminator only (not skipNewlines): a blank
	// line here is the header/body separator and must be preserved so the body
	// isn't eaten by the header loop below.
	if p.curToken.Type == TokenNewline {
		p.nextToken()
	}

	for p.curToken.Type == TokenQueryParam {
		qp, err := p.parseQueryParam()
		if err != nil {
			return nil, err
		}
		req.QueryParams = append(req.QueryParams, qp)
		p.skipNewlines()
	}

	for p.curToken.Type == TokenIdentifier {
		header, err := p.parseHeader()
		if err != nil {
			return nil, err
		}
		if header != nil {
			req.Headers = append(req.Headers, header)
		}
		// Advance past only this header's line terminator. skipNewlines() here
		// consumed the blank line that separates headers from the body, so the
		// loop then ate plain-text body lines (identifier-led, no colon) as
		// failed headers and the body was silently dropped.
		if p.curToken.Type == TokenNewline {
			p.nextToken()
		}
	}

	if p.curToken.Type == TokenNewline {
		p.skipNewlines()
	}

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
			req.Assertions = assertions

		case p.curToken.Type == TokenDBStart:
			dbAssertions, err := p.parseDBBlock()
			if err != nil {
				return nil, err
			}
			req.DBAssertions = dbAssertions

		case p.curToken.Type == TokenShellStart:
			shellCommands, err := p.parseShellBlock()
			if err != nil {
				return nil, err
			}
			req.ShellCommands = shellCommands

		case p.curToken.Type == TokenCaptureStart:
			captures, err := p.parseCaptures()
			if err != nil {
				return nil, err
			}
			req.Captures = captures

		case p.curToken.Type == TokenMockStart:
			mockBody, err := p.parseMockBlock()
			if err != nil {
				return nil, err
			}
			req.MockBody = mockBody

		default:
			return req, nil
		}
	}
}

// parseURL reads the request URL from the current line. It reads the raw line
// (not the token stream) so a URL fragment (#section) isn't comment-stripped by
// the lexer's '#' handling, and a trailing " HTTP/x.y" version token is dropped
// instead of being concatenated into the URL ("http://x/api HTTP/1.1" used to
// become "http://x/apiHTTP/1.1").
func (p *Parser) parseURL() string {
	line := p.lexer.GetCurrentLine()
	// Consume the rest of the request line from the lexer and resync.
	_ = p.lexer.ReadRestOfLine()
	p.curToken = p.lexer.NextToken()

	fields := strings.Fields(line)
	// fields[0] is the method; fields[1] is the URL; an optional fields[2] is
	// the HTTP version (ignored).
	if len(fields) >= 2 {
		return fields[1]
	}
	return ""
}

// parseQueryParam parses a single "? key = value" query parameter line.
func (p *Parser) parseQueryParam() (*QueryParam, error) {
	line := p.curToken.Line
	p.nextToken()
	if p.curToken.Type == TokenWhitespace {
		p.nextToken()
	}

	key := p.curToken.Value
	p.nextToken()

	if p.curToken.Type == TokenWhitespace {
		p.nextToken()
	}
	if p.curToken.Type == TokenEquals {
		p.nextToken()
	}
	if p.curToken.Type == TokenWhitespace {
		p.nextToken()
	}

	var value strings.Builder
	for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenVariableRef {
			value.WriteString("{{")
			value.WriteString(p.curToken.Value)
			value.WriteString("}}")
		} else {
			value.WriteString(p.curToken.Value)
		}
		p.nextToken()
	}

	return &QueryParam{
		Key:   key,
		Value: strings.TrimSpace(value.String()),
		Line:  line,
	}, nil
}

// parseHeader parses a single "Key: Value" header line.
func (p *Parser) parseHeader() (*Header, error) {
	line := p.curToken.Line
	key := p.curToken.Value
	p.nextTokenRaw()

	if p.curToken.Type == TokenWhitespace {
		p.nextTokenRaw()
	}

	if p.curToken.Type != TokenColon {
		return nil, nil
	}
	p.nextTokenRaw()

	if p.curToken.Type == TokenWhitespace {
		p.nextTokenRaw()
	}

	var value strings.Builder
	for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenVariableRef {
			value.WriteString("{{")
			value.WriteString(p.curToken.Value)
			value.WriteString("}}")
		} else if p.curToken.Type == TokenWhitespace {
			value.WriteString(" ")
		} else {
			value.WriteString(p.curToken.Value)
		}
		p.nextTokenRaw()
	}

	return &Header{
		Key:   key,
		Value: strings.TrimSpace(value.String()),
		Line:  line,
	}, nil
}
