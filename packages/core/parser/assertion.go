package parser

import (
	"strconv"
	"strings"
)

// parseAssertions parses a >>> ... <<< assertion block into a slice of Assertion.
func (p *Parser) parseAssertions() ([]*Assertion, error) {
	startLine := p.curToken.Line
	p.nextToken()
	p.skipNewlines()

	var assertions []*Assertion

	// Stop at the closing <<<, end of file, or a new request separator (###).
	// A bare >>> that runs into the next request means the closing <<< was
	// forgotten; without this guard the block silently swallowed every
	// subsequent request and merged their expects into the first one.
	for p.curToken.Type != TokenAssertionEnd &&
		p.curToken.Type != TokenEOF &&
		p.curToken.Type != TokenRequestSeparator {
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

	if p.curToken.Type != TokenAssertionEnd {
		// Reached EOF or a new request without a closing <<< — the block is
		// unclosed. Report it (mirrors >>>mock/>>>graphql) instead of silently
		// consuming the rest of the file.
		return nil, &ParseError{
			File:    p.file,
			Line:    startLine,
			Message: "unclosed >>> assertion block (missing closing <<<)",
			Snippet: p.getSnippet(),
		}
	}
	p.nextToken()

	return assertions, nil
}

// parseAssertion parses a single "expect subject operator expected" line.
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

// parseAssertionSubject reads the assertion subject (e.g., "status", "body.name",
// "header Content-Type"). The "header <name>" subject carries a space-separated
// header name; without consuming it the operator that follows (contains, >,
// exists, ...) would be dropped and the assertion silently became
// "header == <name>".
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
	subject := builder.String()
	// "header <name>": consume the space-separated header name when it is an
	// identifier. Operator keywords (exists/contains/...) are TokenOperator, so
	// "expect header exists" still leaves the subject as "header".
	if strings.EqualFold(subject, "header") && p.curToken.Type == TokenWhitespace {
		p.nextTokenRaw() // advance past the whitespace
		if p.curToken.Type == TokenIdentifier {
			builder.WriteRune(' ')
			builder.WriteString(p.curToken.Value)
			p.nextTokenRaw() // advance past the header name
		}
	}
	return builder.String()
}

// parseAssertionOperator parses the comparison operator in an assertion line.
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
		// Check for compound length operators (length >, length >=, etc.)
		p.skipWhitespace()
		if p.curToken.Type == TokenOperator {
			switch p.curToken.Value {
			case ">":
				p.nextToken()
				return OpLengthGt, nil
			case ">=":
				p.nextToken()
				return OpLengthGte, nil
			case "<":
				p.nextToken()
				return OpLengthLt, nil
			case "<=":
				p.nextToken()
				return OpLengthLte, nil
			}
		}
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
	case "snapshot":
		return OpSnapshot, nil
	}

	return OpEquals, &ParseError{
		File:    p.file,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: "unknown operator: " + op,
		Snippet: p.getSnippet(),
	}
}

// parseAssertionExpected parses the expected value in an assertion line.
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
	case TokenVariableRef:
		// Unquoted {{var}} expected value: keep the {{var}} form so the runner's
		// env resolver interpolates it. Without this it fell to the default branch
		// and was dropped (expected became "").
		v := "{{" + p.curToken.Value + "}}"
		p.nextToken()
		return v
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

// parseArray parses a bracketed array literal [a, b, c].
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
