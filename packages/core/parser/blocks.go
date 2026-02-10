package parser

import (
	"strconv"
	"strings"
)

// parseDBBlock parses a >>>db ... <<< block containing SQL queries and expectations.
func (p *Parser) parseDBBlock() ([]*DBAssertion, error) {
	p.nextToken()
	p.skipNewlines()

	var assertions []*DBAssertion
	var currentQuery string
	var queryLine int

	for p.curToken.Type != TokenAssertionEnd && p.curToken.Type != TokenEOF {
		line := p.curToken.Line

		// Read the line
		lineContent := p.curToken.Value
		for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF && p.curToken.Type != TokenAssertionEnd {
			p.nextToken()
			if p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF && p.curToken.Type != TokenAssertionEnd {
				lineContent += " " + p.curToken.Value
			}
		}
		lineContent = strings.TrimSpace(lineContent)

		if strings.HasPrefix(lineContent, "query ") {
			currentQuery = strings.TrimPrefix(lineContent, "query ")
			currentQuery = strings.TrimSpace(currentQuery)
			queryLine = line
		} else if strings.HasPrefix(lineContent, "expect ") && currentQuery != "" {
			// Parse the expect line: expect column operator expected
			expectPart := strings.TrimPrefix(lineContent, "expect ")
			expectPart = strings.TrimSpace(expectPart)

			assertion, err := p.parseDBExpectLine(expectPart, currentQuery, queryLine)
			if err != nil {
				return nil, err
			}
			assertions = append(assertions, assertion)
		}

		if p.curToken.Type == TokenNewline {
			p.nextToken()
		}
		p.skipNewlines()
	}

	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}

	return assertions, nil
}

// parseDBExpectLine parses a single expect line within a >>>db block.
func (p *Parser) parseDBExpectLine(line string, query string, queryLine int) (*DBAssertion, error) {
	// Parse: column operator expected
	// Example: count > 0, name == "John"
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, &ParseError{
			File:    p.file,
			Line:    queryLine,
			Message: "invalid db expect syntax: " + line,
			Snippet: p.getSnippet(),
		}
	}

	column := parts[0]
	operator := OpEquals
	var expected interface{}

	if len(parts) >= 3 {
		// Parse operator
		switch parts[1] {
		case "==":
			operator = OpEquals
		case "!=":
			operator = OpNotEquals
		case ">":
			operator = OpGreaterThan
		case ">=":
			operator = OpGreaterOrEqual
		case "<":
			operator = OpLessThan
		case "<=":
			operator = OpLessOrEqual
		case "contains":
			operator = OpContains
		case "exists":
			operator = OpExists
		case "!exists":
			operator = OpNotExists
		}

		// Parse expected value
		expectedStr := strings.Join(parts[2:], " ")
		expected = parseValue(expectedStr)
	} else if len(parts) == 2 {
		// Shorthand: column expected (implies ==)
		expected = parseValue(parts[1])
	}

	return &DBAssertion{
		Query:    query,
		Column:   column,
		Operator: operator,
		Expected: expected,
		Line:     queryLine,
	}, nil
}

// parseValue parses a literal value string into a typed Go value.
func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Check for quoted string
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	// Check for number
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Check for boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Check for null
	if s == "null" {
		return nil
	}

	return s
}

// parseShellBlock parses a >>>shell ... <<< block into a slice of ShellCommand.
func (p *Parser) parseShellBlock() ([]*ShellCommand, error) {
	p.nextToken()
	p.skipNewlines()

	var commands []*ShellCommand

	for p.curToken.Type != TokenAssertionEnd && p.curToken.Type != TokenEOF {
		line := p.curToken.Line

		// Build the command by reading from current token to end of line
		var cmdBuilder strings.Builder
		for p.curToken.Type != TokenNewline && p.curToken.Type != TokenEOF && p.curToken.Type != TokenAssertionEnd {
			if cmdBuilder.Len() > 0 && p.curToken.Type == TokenWhitespace {
				cmdBuilder.WriteString(" ")
			} else if p.curToken.Type == TokenVariableRef {
				cmdBuilder.WriteString("{{")
				cmdBuilder.WriteString(p.curToken.Value)
				cmdBuilder.WriteString("}}")
			} else if p.curToken.Type != TokenWhitespace {
				cmdBuilder.WriteString(p.curToken.Value)
			}
			p.nextTokenRaw()
		}

		cmd := strings.TrimSpace(cmdBuilder.String())
		if cmd != "" {
			commands = append(commands, &ShellCommand{
				Command: cmd,
				Line:    line,
			})
		}

		if p.curToken.Type == TokenNewline {
			p.nextToken()
		}
		p.skipNewlines()
	}

	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}

	return commands, nil
}
