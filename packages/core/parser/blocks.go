package parser

import (
	"strings"
)

// dbOperators are the operators a >>>db expectation supports.
var dbOperators = map[AssertionOperator]bool{
	OpEquals: true, OpNotEquals: true, OpGreaterThan: true, OpGreaterOrEqual: true,
	OpLessThan: true, OpLessOrEqual: true, OpContains: true, OpExists: true, OpNotExists: true,
}

// parseDBBlock parses a >>>db ... <<< block: SQL and "expect column op value"
// lines. The SQL is read raw, so quotes, braces ({{var}}) and punctuation
// reach the database exactly as written.
//
// A query is "query <sql>" or a bare SQL statement; a line that follows
// another query line and is not an expect line continues the statement, and a
// SQL line after an expect line starts the next query.
func (p *Parser) parseDBBlock() ([]*DBAssertion, error) {
	var assertions []*DBAssertion
	var query string
	var queryLine int
	queryHasExpects := false

	err := p.readBlock("db", func(lineNo int, raw string) error {
		text := strings.TrimSpace(raw)
		lower := strings.ToLower(text)
		switch {
		case lower == "expect" || strings.HasPrefix(lower, "expect ") || strings.HasPrefix(lower, "expect\t"):
			if query == "" {
				return (&exprScanner{p: p, line: lineNo, text: raw}).fail(strings.Index(raw, text), "expect before any query: put the SQL on a line before it")
			}
			a, err := p.parseDBExpectLine(lineNo, raw, query, queryLine)
			if err != nil {
				return err
			}
			assertions = append(assertions, a)
			queryHasExpects = true
		case strings.HasPrefix(lower, "query ") || strings.HasPrefix(lower, "query\t") || lower == "query":
			query = strings.TrimSpace(text[len("query"):])
			queryLine = lineNo
			queryHasExpects = false
		case query == "" || queryHasExpects:
			query, queryLine, queryHasExpects = text, lineNo, false
		default:
			query += "\n" + text
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return assertions, nil
}

// parseDBExpectLine parses "expect <column> <operator> <value>" inside a >>>db
// block with the same grammar as an HTTP assertion.
func (p *Parser) parseDBExpectLine(lineNo int, raw string, query string, queryLine int) (*DBAssertion, error) {
	s := &exprScanner{p: p, line: lineNo, text: raw}
	if err := s.expectKeyword("expect"); err != nil {
		return nil, err
	}
	s.skipSpace()
	column := s.scanFullSubject(false)
	if column == "" {
		return nil, s.fail(s.pos, "missing column after expect")
	}
	opPos := s.pos
	op, expected, err := s.parseCondition(column, false)
	if err != nil {
		return nil, err
	}
	if !dbOperators[op] {
		return nil, s.fail(opPos, "operator %s is not supported in >>>db blocks (use ==, !=, >, >=, <, <=, contains, exists, !exists)", op.String())
	}
	return &DBAssertion{
		Query:    query,
		Column:   column,
		Operator: op,
		Expected: expected,
		Line:     queryLine,
	}, nil
}

// parseMockBlock parses a >>>mock ... <<< block, capturing the raw response body
// the mock server should return for this request. Mirrors the GraphQL block's
// raw-read so multi-line JSON is preserved verbatim.
func (p *Parser) parseMockBlock() (string, error) {
	startLine := p.curToken.Line
	// Advance past >>>mock; lexer is at the newline after the block type.
	p.nextToken()
	body := p.lexer.ReadRawUntilBlockEnd()
	// Re-sync the token stream after the raw read (lexer is at <<< or EOF).
	p.curToken = p.lexer.NextToken()
	if p.curToken.Type == TokenEOF {
		return "", &ParseError{
			File:    p.file,
			Line:    startLine,
			Message: "unclosed >>>mock block (missing closing <<<)",
			Snippet: p.getSnippet(),
		}
	}
	if p.curToken.Type == TokenAssertionEnd {
		p.nextToken()
	}
	return strings.TrimSpace(body), nil
}

// parseShellBlock parses a >>>shell ... <<< block: one command per line, read
// raw so quotes, repeated spaces and '#' reach the shell exactly as written.
// Blank lines and lines starting with '#' or '//' are skipped.
func (p *Parser) parseShellBlock() ([]*ShellCommand, error) {
	var commands []*ShellCommand
	err := p.readBlock("shell", func(lineNo int, raw string) error {
		commands = append(commands, &ShellCommand{Command: strings.TrimSpace(raw), Line: lineNo})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return commands, nil
}
