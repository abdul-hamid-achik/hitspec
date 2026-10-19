package parser

import (
	"fmt"
	"strings"
)

// readBlock reads the raw lines of a >>> block up to its closing <<<, calling
// fn for every line that is not blank or a comment. The current token must be
// the block-start token; afterwards the token stream continues after <<<.
//
// An unclosed block (end of file, or a new ### / >>> line before <<<) is an
// error: it used to swallow the rest of the file.
func (p *Parser) readBlock(kind string, fn func(lineNo int, raw string) error) error {
	start := p.curToken
	snippet := p.getSnippet()
	l := p.lexer
	l.skipLine() // the rest of the >>> line
	name := ">>>" + kind
	if kind == "" {
		name = ">>> assertion"
	}
	unclosed := func() error {
		return &ParseError{
			File:    p.file,
			Line:    start.Line,
			Column:  start.Column,
			Message: fmt.Sprintf("unclosed %s block (missing closing <<<)", name),
			Snippet: snippet,
		}
	}
	for {
		if l.atEOF() {
			return unclosed()
		}
		lineNo := l.line
		raw := l.peekLine()
		t := strings.TrimSpace(raw)
		switch {
		case t == "":
			l.skipLine()
		case strings.HasPrefix(t, "<<<"):
			l.skipLine()
			p.nextToken()
			return nil
		case strings.HasPrefix(t, "###"), strings.HasPrefix(t, ">>>"):
			return unclosed()
		case strings.HasPrefix(t, "#"), strings.HasPrefix(t, "//"):
			l.skipLine()
		default:
			if err := fn(lineNo, raw); err != nil {
				return err
			}
			l.skipLine()
		}
	}
}

// parseAssertions parses a >>> ... <<< assertion block into a slice of Assertion.
func (p *Parser) parseAssertions() ([]*Assertion, error) {
	var assertions []*Assertion
	err := p.readBlock("", func(lineNo int, raw string) error {
		a, err := p.parseExpectLine(lineNo, raw)
		if err != nil {
			return err
		}
		assertions = append(assertions, a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return assertions, nil
}

// parseExpectLine parses one "expect <subject> <operator> <value>" line.
func (p *Parser) parseExpectLine(lineNo int, raw string) (*Assertion, error) {
	s := &exprScanner{p: p, line: lineNo, text: raw}
	if err := s.expectKeyword("expect"); err != nil {
		return nil, err
	}
	s.skipSpace()
	subject := s.scanFullSubject(true)
	if subject == "" {
		return nil, s.fail(s.pos, "missing subject after expect (e.g. expect status 200)")
	}
	subject = normalizeSubjectKeyword(subject)
	op, expected, err := s.parseCondition(subject, true)
	if err != nil {
		return nil, err
	}
	return &Assertion{
		Subject:  subject,
		Operator: op,
		Expected: expected,
		Line:     lineNo,
	}, nil
}

// expectKeyword consumes the leading keyword of a block line. Anything else is
// an error, so a misspelt "expcet status 200" fails instead of asserting nothing.
func (s *exprScanner) expectKeyword(want string) error {
	s.skipSpace()
	word := s.peekWord()
	if strings.EqualFold(word, want) {
		s.pos += len(word)
		return nil
	}
	msg := fmt.Sprintf("unexpected line in block: expected %q, got %q", want+" ...", word)
	if hint := closest(word, []string{want}, true); hint != "" {
		msg = fmt.Sprintf("unexpected line in block: %q (did you mean %q?)", word, want)
	}
	return s.fail(s.pos, "%s", msg)
}

// normalizeSubjectKeyword lowercases the "header" / "jsonpath" keyword of a
// subject; the argument keeps its case.
func normalizeSubjectKeyword(subject string) string {
	for _, kw := range []string{"header", "jsonpath"} {
		if len(subject) >= len(kw) && strings.EqualFold(subject[:len(kw)], kw) &&
			(len(subject) == len(kw) || subject[len(kw)] == ' ') {
			return kw + subject[len(kw):]
		}
	}
	return subject
}
