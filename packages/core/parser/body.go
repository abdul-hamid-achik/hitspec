package parser

import (
	"regexp"
	"strings"
)

// parseBody parses the request body, dispatching to specialized parsers
// for multipart, GraphQL, and form block bodies.
func (p *Parser) parseBody() (*Body, error) {
	line := p.curToken.Line

	if p.curToken.Type == TokenMultipartStart {
		return p.parseMultipartBody()
	}
	if p.curToken.Type == TokenGraphQLStart {
		return p.parseGraphQLBody()
	}
	if p.curToken.Type == TokenQueryParam && p.curToken.Value == "&" {
		return p.parseFormBlockBody()
	}

	raw := p.readRawBody()
	if raw == "" {
		return nil, nil
	}

	// File-based body: < ./path/to/file.json
	if strings.HasPrefix(raw, "< ") {
		filePath := strings.TrimSpace(raw[2:])
		return &Body{
			ContentType: BodyFile,
			FilePath:    filePath,
			Raw:         raw,
			Line:        line,
		}, nil
	}

	body := &Body{
		Raw:  raw,
		Line: line,
	}

	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		body.ContentType = BodyJSON
	} else if strings.HasPrefix(raw, "<?xml") {
		body.ContentType = BodyXML
	} else if strings.Contains(raw, "=") && !strings.Contains(raw, "\n") {
		body.ContentType = BodyForm
	} else {
		body.ContentType = BodyRaw
	}

	return body, nil
}

// parseFormBlockBody parses a form body written with "& key = value" lines. It
// reads raw lines so spaces inside values survive; the fields are kept apart in
// Body.FormFields so the HTTP layer can URL-encode them, and Body.Raw is the
// unencoded "key=value&key=value" form.
func (p *Parser) parseFormBlockBody() (*Body, error) {
	line := p.curToken.Line
	l := p.lexer
	l.seekLineStart(p.curToken.Offset, p.curToken.Line)

	var fields []*QueryParam
	var raw []string
	for !l.atEOF() {
		text := strings.TrimSpace(l.peekLine())
		if text == "" {
			// Blank lines between fields are allowed.
			l.skipLine()
			continue
		}
		if text[0] != '&' {
			break
		}
		f := parseQueryParamLine(text, l.line)
		fields = append(fields, f)
		raw = append(raw, f.Key+"="+f.Value)
		l.skipLine()
	}
	p.nextToken()

	return &Body{
		ContentType: BodyFormBlock,
		Raw:         strings.Join(raw, "&"),
		FormFields:  fields,
		Line:        line,
	}, nil
}

// parseMultipartBody parses a >>>multipart ... <<< block.
func (p *Parser) parseMultipartBody() (*Body, error) {
	line := p.curToken.Line
	p.nextToken()
	p.skipNewlines()

	body := &Body{
		ContentType: BodyMultipart,
		Line:        line,
	}

	for p.curToken.Type != TokenAssertionEnd && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenIdentifier {
			fieldType := strings.ToLower(p.curToken.Value)
			field := &MultipartField{}

			// Use raw lexer reading so spaces in values are preserved
			l := p.lexer
			l.skipWhitespaceInLine()

			if fieldType == "file" {
				field.Type = MultipartFieldFile
				field.Name, field.Path = splitMultipartFile(l.ReadRestOfLine())
			} else if fieldType == "field" {
				field.Type = MultipartFieldValue
				// Read the field name
				var nameBuilder strings.Builder
				for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '-' || l.ch == '.' {
					nameBuilder.WriteRune(l.ch)
					l.readChar()
				}
				field.Name = nameBuilder.String()
				l.skipWhitespaceInLine()
				if l.ch == '=' {
					l.readChar()
				}
				field.Value = l.ReadRestOfLine()
			}

			body.Multipart = append(body.Multipart, field)
		}
		p.nextToken()
		p.skipNewlines()
	}

	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}

	return body, nil
}

// defaultMultipartFileName is the form field name of a file part that does not
// name one.
const defaultMultipartFileName = "file"

// splitMultipartFile splits the rest of a "file" line in a >>>multipart block:
//
//	file <name> @<path>   the part is sent as form field <name>
//	file @<path>          field name "file"
//	file <path>           field name "file" (the @ is optional)
//
// A first word followed by "@..." is the field name; anything else is a path,
// so a path with spaces still works.
func splitMultipartFile(rest string) (name, path string) {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "@") {
		return defaultMultipartFileName, strings.TrimSpace(rest[1:])
	}
	if i := strings.IndexAny(rest, " \t"); i > 0 {
		if after := strings.TrimSpace(rest[i+1:]); strings.HasPrefix(after, "@") {
			return rest[:i], strings.TrimSpace(after[1:])
		}
	}
	return defaultMultipartFileName, rest
}

// parseGraphQLBody parses a >>>graphql ... <<< block with optional >>>variables block.
func (p *Parser) parseGraphQLBody() (*Body, error) {
	line := p.curToken.Line
	// Advance past the >>>graphql token; lexer is now at \n after block type.
	// nextToken() consumes the \n, positioning the lexer at the content start.
	// Do NOT call skipNewlines() — it would consume the first content token.
	p.nextToken()

	body := &Body{
		ContentType: BodyGraphQL,
		GraphQL:     &GraphQLBody{},
		Line:        line,
	}

	query := p.lexer.ReadRawUntilBlockEnd()
	body.GraphQL.Query = query

	// Re-sync token stream after raw read (lexer is at >>>, <<<, or EOF)
	p.curToken = p.lexer.NextToken()

	if p.curToken.Type == TokenEOF {
		return nil, &ParseError{
			File:    p.file,
			Line:    line,
			Message: "unclosed >>>graphql block (missing closing <<<)",
			Snippet: p.getSnippet(),
		}
	}
	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}
	p.skipNewlines()

	if p.curToken.Type == TokenVariablesStart {
		// Advance past >>>variables; do NOT skipNewlines to preserve content.
		p.nextToken()
		vars := p.lexer.ReadRawUntilBlockEnd()
		body.GraphQL.Variables = vars
		// Re-sync token stream after raw read
		p.curToken = p.lexer.NextToken()
		if p.curToken.Type == TokenAssertionEnd {
			p.nextToken()
		}
	}

	return body, nil
}

// varDefLineRe matches a file-level "@name = value" variable definition line.
var varDefLineRe = regexp.MustCompile(`^@[\p{L}\p{N}_.\-]*[ \t]*=`)

// readRawBody reads the request body verbatim, line by line, starting at the
// line holding the current token, and resyncs the token stream at the line that
// ends it. The body ends at a request separator, a >>> block marker or a
// "@name = value" variable line. Nothing inside is interpreted: quotes,
// apostrophes, '#', '@' and '>>>' mid-line are all body text. Trailing comment
// lines (a note placed before the assertion block) are not part of the body.
func (p *Parser) readRawBody() string {
	l := p.lexer
	l.seekLineStart(p.curToken.Offset, p.curToken.Line)
	var lines []string
	for !l.atEOF() {
		text := l.peekLine()
		t := strings.TrimSpace(text)
		if strings.HasPrefix(t, "###") || strings.HasPrefix(t, ">>>") || varDefLineRe.MatchString(t) {
			break
		}
		lines = append(lines, text)
		l.skipLine()
	}
	for len(lines) > 0 {
		t := strings.TrimSpace(lines[len(lines)-1])
		if t != "" && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "//") {
			break
		}
		lines = lines[:len(lines)-1]
	}
	p.nextToken()
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
