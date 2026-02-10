package parser

import "strings"

// parseCaptures parses a >>>capture ... <<< block into a slice of Capture.
func (p *Parser) parseCaptures() ([]*Capture, error) {
	p.nextToken()
	p.skipNewlines()

	var captures []*Capture

	for p.curToken.Type != TokenAssertionEnd && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenIdentifier {
			capture, err := p.parseCapture()
			if err != nil {
				return nil, err
			}
			captures = append(captures, capture)
		} else {
			p.nextToken()
		}
		p.skipNewlines()
	}

	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}

	return captures, nil
}

// parseCapture parses a single "name from source.path" capture line.
func (p *Parser) parseCapture() (*Capture, error) {
	line := p.curToken.Line
	name := p.curToken.Value
	p.nextToken()

	if p.curToken.Type == TokenWhitespace {
		p.nextToken()
	}

	if p.curToken.Type != TokenFrom {
		return nil, &ParseError{
			File:    p.file,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected 'from', got " + p.curToken.Value,
			Snippet: p.getSnippet(),
		}
	}
	p.nextTokenRaw()
	p.skipWhitespace()

	var pathBuilder strings.Builder
	for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenWhitespace {
			pathBuilder.WriteString(" ")
		} else {
			pathBuilder.WriteString(p.curToken.Value)
		}
		p.nextTokenRaw()
	}
	path := strings.TrimSpace(pathBuilder.String())

	source := CaptureBody
	if strings.HasPrefix(path, "header ") || path == "header" {
		source = CaptureHeader
		path = strings.TrimPrefix(path, "header")
		path = strings.TrimSpace(path)
	} else if strings.HasPrefix(path, "body.") {
		path = strings.TrimPrefix(path, "body.")
	} else if strings.HasPrefix(path, "body") && len(path) > 4 && path[4] == '[' {
		path = strings.TrimPrefix(path, "body")
	} else if path == "status" {
		source = CaptureStatus
		path = ""
	} else if path == "duration" {
		source = CaptureDuration
		path = ""
	}

	p.nextToken()

	return &Capture{
		Name:   name,
		Source: source,
		Path:   path,
		Line:   line,
	}, nil
}
