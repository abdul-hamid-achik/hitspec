package parser

import (
	"os"
	"strconv"
	"strings"
)

type Parser struct {
	lexer    *Lexer
	curToken Token
	errors   []*ParseError
	file     string
}

func NewParser(input string) *Parser {
	p := &Parser{
		lexer: NewLexer(input),
	}
	p.nextToken()
	return p
}

func ParseFile(path string) (*File, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(content), path)
}

func Parse(input, filename string) (*File, error) {
	p := NewParser(input)
	p.file = filename
	return p.ParseFile()
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

func (p *Parser) ParseFile() (*File, error) {
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
		p.curToken.Type != TokenRequestSeparator &&
		p.curToken.Type != TokenEOF {
		body, err := p.parseBody()
		if err != nil {
			return nil, err
		}
		req.Body = body
	}

	p.skipNewlines()

	if p.curToken.Type == TokenAssertionStart && p.curToken.Value == "" {
		assertions, err := p.parseAssertions()
		if err != nil {
			return nil, err
		}
		req.Assertions = assertions
		p.skipNewlines()
	}

	if p.curToken.Type == TokenCaptureStart {
		captures, err := p.parseCaptures()
		if err != nil {
			return nil, err
		}
		req.Captures = captures
	}

	return req, nil
}

func (p *Parser) parseAnnotation(req *Request) error {
	name := strings.ToLower(p.curToken.Value)
	value := ""
	if p.curToken.Literal != nil {
		value = p.curToken.Literal.(string)
	}

	switch name {
	case "name":
		if req.Name == "" {
			req.Name = value
		}
	case "description":
		req.Description = value
	case "tags":
		tags := strings.Split(value, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				req.Tags = append(req.Tags, t)
			}
		}
	case "skip":
		req.Metadata.Skip = value
		if req.Metadata.Skip == "" {
			req.Metadata.Skip = "skipped"
		}
	case "only":
		req.Metadata.Only = true
	case "timeout":
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.Timeout = v
		}
	case "retry":
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.Retry = v
		}
	case "retrydelay":
		if v, err := strconv.Atoi(value); err == nil {
			req.Metadata.RetryDelay = v
		}
	case "depends":
		deps := strings.Split(value, ",")
		for _, d := range deps {
			d = strings.TrimSpace(d)
			if d != "" {
				req.Metadata.Depends = append(req.Metadata.Depends, d)
			}
		}
	case "auth":
		auth, err := parseAuthConfig(value)
		if err != nil {
			return err
		}
		req.Metadata.Auth = auth
	}

	return nil
}

func parseAuthConfig(value string) (*AuthConfig, error) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return nil, nil
	}

	auth := &AuthConfig{}
	switch strings.ToLower(parts[0]) {
	case "basic":
		auth.Type = AuthBasic
		auth.Params = parts[1:]
	case "bearer":
		auth.Type = AuthBearer
		auth.Params = parts[1:]
	case "apikey":
		auth.Type = AuthAPIKey
		auth.Params = parts[1:]
	case "apikey-query":
		auth.Type = AuthAPIKeyQuery
		auth.Params = parts[1:]
	case "digest":
		auth.Type = AuthDigest
		auth.Params = parts[1:]
	case "aws":
		auth.Type = AuthAWS
		auth.Params = parts[1:]
	}

	return auth, nil
}

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

	var builder strings.Builder
	for p.curToken.Type != TokenAssertionStart &&
		p.curToken.Type != TokenCaptureStart &&
		p.curToken.Type != TokenRequestSeparator &&
		p.curToken.Type != TokenEOF {

		if p.curToken.Type == TokenVariableRef {
			builder.WriteString("{{")
			builder.WriteString(p.curToken.Value)
			builder.WriteString("}}")
		} else if p.curToken.Type == TokenNewline {
			builder.WriteString("\n")
		} else {
			builder.WriteString(p.curToken.Value)
		}
		p.nextToken()
	}

	raw := strings.TrimSpace(builder.String())
	if raw == "" {
		return nil, nil
	}

	body := &Body{
		Raw:  raw,
		Line: line,
	}

	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		body.ContentType = BodyJSON
	} else if strings.HasPrefix(raw, "<?xml") || strings.HasPrefix(raw, "<") {
		body.ContentType = BodyXML
	} else if strings.Contains(raw, "=") && !strings.Contains(raw, "\n") {
		body.ContentType = BodyForm
	} else {
		body.ContentType = BodyRaw
	}

	return body, nil
}

func (p *Parser) parseFormBlockBody() (*Body, error) {
	line := p.curToken.Line
	var fields []string

	for p.curToken.Type == TokenQueryParam && p.curToken.Value == "&" {
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

		fields = append(fields, key+"="+strings.TrimSpace(value.String()))
		p.skipNewlines()
	}

	return &Body{
		ContentType: BodyFormBlock,
		Raw:         strings.Join(fields, "&"),
		Line:        line,
	}, nil
}

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
			p.nextToken()
			if p.curToken.Type == TokenWhitespace {
				p.nextToken()
			}

			field := &MultipartField{}

			if fieldType == "file" {
				field.Type = MultipartFieldFile
				if p.curToken.Value == "@" {
					p.nextToken()
				}
				field.Path = p.lexer.ReadRestOfLine()
			} else if fieldType == "field" {
				field.Type = MultipartFieldValue
				field.Name = p.curToken.Value
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
				field.Value = p.lexer.ReadRestOfLine()
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

func (p *Parser) parseGraphQLBody() (*Body, error) {
	line := p.curToken.Line
	p.nextToken()
	p.skipNewlines()

	body := &Body{
		ContentType: BodyGraphQL,
		GraphQL:     &GraphQLBody{},
		Line:        line,
	}

	query := p.lexer.ReadRawUntilBlockEnd()
	body.GraphQL.Query = query

	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}
	p.skipNewlines()

	if p.curToken.Type == TokenVariablesStart {
		p.nextToken()
		p.skipNewlines()
		vars := p.lexer.ReadRawUntilBlockEnd()
		body.GraphQL.Variables = vars
		if p.curToken.Type == TokenAssertionEnd {
			p.nextToken()
		}
	}

	return body, nil
}

func (p *Parser) parseAssertions() ([]*Assertion, error) {
	p.nextToken()
	p.skipNewlines()

	var assertions []*Assertion

	for p.curToken.Type != TokenAssertionEnd && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenExpect {
			assertion, err := p.parseAssertion()
			if err != nil {
				return nil, err
			}
			assertions = append(assertions, assertion)
		} else {
			p.nextToken()
		}
		p.skipNewlines()
	}

	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}

	return assertions, nil
}

func (p *Parser) parseAssertion() (*Assertion, error) {
	line := p.curToken.Line
	p.nextTokenRaw()
	p.skipWhitespace()

	subject := p.parseAssertionSubject()
	p.skipWhitespace()

	operator, err := p.parseAssertionOperator()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()

	var expected any
	if operator != OpExists && operator != OpNotExists {
		expected = p.parseAssertionExpected()
	}

	return &Assertion{
		Subject:  subject,
		Operator: operator,
		Expected: expected,
		Line:     line,
	}, nil
}

func (p *Parser) parseAssertionSubject() string {
	var builder strings.Builder
	for p.curToken.Type != TokenWhitespace &&
		p.curToken.Type != TokenNewline &&
		p.curToken.Type != TokenEOF &&
		p.curToken.Type != TokenOperator {

		if p.curToken.Type == TokenVariableRef {
			builder.WriteString("{{")
			builder.WriteString(p.curToken.Value)
			builder.WriteString("}}")
		} else {
			builder.WriteString(p.curToken.Value)
		}
		p.nextTokenRaw()
	}
	return builder.String()
}

func (p *Parser) parseAssertionOperator() (AssertionOperator, error) {
	if p.curToken.Type != TokenOperator {
		return OpEquals, nil
	}

	op := strings.ToLower(p.curToken.Value)
	p.nextToken()

	switch op {
	case "==":
		return OpEquals, nil
	case "!=":
		return OpNotEquals, nil
	case ">":
		return OpGreaterThan, nil
	case ">=":
		return OpGreaterOrEqual, nil
	case "<":
		return OpLessThan, nil
	case "<=":
		return OpLessOrEqual, nil
	case "contains":
		return OpContains, nil
	case "!contains":
		return OpNotContains, nil
	case "startswith":
		return OpStartsWith, nil
	case "endswith":
		return OpEndsWith, nil
	case "matches":
		return OpMatches, nil
	case "exists":
		return OpExists, nil
	case "!exists":
		return OpNotExists, nil
	case "length":
		return OpLength, nil
	case "includes":
		return OpIncludes, nil
	case "!includes":
		return OpNotIncludes, nil
	case "in":
		return OpIn, nil
	case "!in":
		return OpNotIn, nil
	case "type":
		return OpType, nil
	case "each":
		return OpEach, nil
	case "schema":
		return OpSchema, nil
	}

	return OpEquals, &ParseError{
		File:    p.file,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: "unknown operator: " + op,
	}
}

func (p *Parser) parseAssertionExpected() any {
	switch p.curToken.Type {
	case TokenString:
		v := p.curToken.Literal
		p.nextToken()
		return v
	case TokenNumber:
		v := p.curToken.Value
		p.nextToken()
		if strings.Contains(v, ".") {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		return v
	case TokenBoolean:
		v := p.curToken.Literal
		p.nextToken()
		return v
	case TokenNull:
		p.nextToken()
		return nil
	case TokenLeftBracket:
		return p.parseArray()
	case TokenIdentifier:
		v := p.curToken.Value
		p.nextToken()
		return v
	default:
		v := p.lexer.ReadRestOfLine()
		p.nextToken()
		return strings.TrimSpace(v)
	}
}

func (p *Parser) parseArray() []any {
	p.nextToken()
	var arr []any
	for p.curToken.Type != TokenRightBracket && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenWhitespace || p.curToken.Type == TokenComma {
			p.nextToken()
			continue
		}
		arr = append(arr, p.parseAssertionExpected())
	}
	if p.curToken.Type == TokenRightBracket {
		p.nextToken()
	}
	return arr
}

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
		}
	}
	p.nextTokenRaw()
	p.skipWhitespace()

	var pathBuilder strings.Builder
	for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF {
		if p.curToken.Type == TokenWhitespace {
			break
		}
		pathBuilder.WriteString(p.curToken.Value)
		p.nextTokenRaw()
	}
	path := strings.TrimSpace(pathBuilder.String())

	source := CaptureBody
	if strings.HasPrefix(path, "header") {
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
