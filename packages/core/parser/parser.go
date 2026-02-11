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

// parseFile parses the top-level file structure: variables followed by requests.
func (p *Parser) parseFile() (*File, error) {
	file := &File{Path: p.file}
	p.skipNewlines()

	for p.curToken.Type == TokenVariable {
		v := &Variable{
			Name:  p.curToken.Value,
			Value: p.curToken.Literal.(string),
			Line:  p.curToken.Line,
		}
		file.Variables = append(file.Variables, v)
		p.nextToken()
		p.skipNewlines()
	}

	for p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenRequestSeparator {
			req, err := p.parseRequest()
			if err != nil {
				return nil, err
			}
			file.Requests = append(file.Requests, req)
		} else if p.curToken.Type == TokenMethod {
			req, err := p.parseRequest()
			if err != nil {
				return nil, err
			}
			file.Requests = append(file.Requests, req)
		} else {
			p.nextToken()
		}
		p.skipNewlines()
	}

	return file, nil
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

	p.skipNewlines()

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
		p.skipNewlines()
	}

	if p.curToken.Type == TokenNewline {
		p.skipNewlines()
	}

	if p.curToken.Type != TokenAssertionStart &&
		p.curToken.Type != TokenCaptureStart &&
		p.curToken.Type != TokenDBStart &&
		p.curToken.Type != TokenShellStart &&
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

		default:
			return req, nil
		}
	}
}

// parseURL reads tokens until end-of-line to build the request URL string.
func (p *Parser) parseURL() string {
	var builder strings.Builder
	for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenVariableRef {
			builder.WriteString("{{")
			builder.WriteString(p.curToken.Value)
			builder.WriteString("}}")
		} else {
			builder.WriteString(p.curToken.Value)
		}
		p.nextToken()
	}
	return strings.TrimSpace(builder.String())
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
