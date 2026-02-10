package parser

import (
	"strconv"
	"strings"
)

// parseAssertions parses a >>> ... <<< assertion block into a slice of Assertion.
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

// parseAssertionSubject reads the assertion subject (e.g., "status", "body.name").
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
