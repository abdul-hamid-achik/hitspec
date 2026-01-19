package parser

import (
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenNewline
	TokenWhitespace
	TokenComment
	TokenRequestSeparator
	TokenMethod
	TokenURL
	TokenHeaderKey
	TokenHeaderValue
	TokenColon
	TokenEquals
	TokenVariable
	TokenVariableRef
	TokenAnnotation
	TokenAnnotationValue
	TokenAssertionStart
	TokenAssertionEnd
	TokenCaptureStart
	TokenCaptureEnd
	TokenExpect
	TokenFrom
	TokenText
	TokenNumber
	TokenString
	TokenBoolean
	TokenNull
	TokenOperator
	TokenQueryParam
	TokenBodyStart
	TokenMultipartStart
	TokenGraphQLStart
	TokenVariablesStart
	TokenDBStart
	TokenShellStart
	TokenIdentifier
	TokenLeftBracket
	TokenRightBracket
	TokenLeftParen
	TokenRightParen
	TokenComma
	TokenDot
	TokenAsterisk
)

type Token struct {
	Type    TokenType
	Value   string
	Line    int
	Column  int
	Literal any
}

type Lexer struct {
	input   string
	pos     int
	readPos int
	ch      byte
	line    int
	column  int
}

func NewLexer(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.column++
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) peekChars(n int) string {
	end := l.readPos + n - 1
	if end > len(l.input) {
		end = len(l.input)
	}
	return l.input[l.pos:end]
}

func (l *Lexer) NextToken() Token {
	var tok Token
	tok.Line = l.line
	tok.Column = l.column

	switch l.ch {
	case 0:
		tok.Type = TokenEOF
		tok.Value = ""
	case '\n':
		tok.Type = TokenNewline
		tok.Value = "\n"
		l.readChar()
	case '\r':
		l.readChar()
		if l.ch == '\n' {
			tok.Type = TokenNewline
			tok.Value = "\n"
			l.readChar()
		}
	case '#':
		if l.peekChars(3) == "###" {
			tok = l.readRequestSeparator()
		} else if l.isAnnotationLine() {
			tok = l.readCommentAnnotation()
		} else {
			tok = l.readLineComment()
		}
	case '/':
		if l.peekChar() == '/' && l.column == 1 {
			tok = l.readLineComment()
		} else {
			tok.Type = TokenText
			tok.Value = string(l.ch)
			l.readChar()
		}
	case '@':
		tok = l.readAnnotationOrVariable()
	case ':':
		tok.Type = TokenColon
		tok.Value = ":"
		l.readChar()
	case '=':
		if l.peekChar() == '=' {
			tok.Type = TokenOperator
			tok.Value = "=="
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TokenEquals
			tok.Value = "="
			l.readChar()
		}
	case '!':
		if l.peekChar() == '=' {
			tok.Type = TokenOperator
			tok.Value = "!="
			l.readChar()
			l.readChar()
		} else {
			tok = l.readOperatorWord()
		}
	case '>':
		if l.peekChars(3) == ">>>" {
			tok = l.readBlockStart()
		} else if l.peekChar() == '=' {
			tok.Type = TokenOperator
			tok.Value = ">="
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TokenOperator
			tok.Value = ">"
			l.readChar()
		}
	case '<':
		if l.peekChars(3) == "<<<" {
			tok = l.readBlockEnd()
		} else if l.peekChar() == '=' {
			tok.Type = TokenOperator
			tok.Value = "<="
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TokenOperator
			tok.Value = "<"
			l.readChar()
		}
	case '{':
		if l.peekChar() == '{' {
			tok = l.readVariableRef()
		} else {
			tok.Type = TokenText
			tok.Value = string(l.ch)
			l.readChar()
		}
	case '[':
		tok.Type = TokenLeftBracket
		tok.Value = "["
		l.readChar()
	case ']':
		tok.Type = TokenRightBracket
		tok.Value = "]"
		l.readChar()
	case '(':
		tok.Type = TokenLeftParen
		tok.Value = "("
		l.readChar()
	case ')':
		tok.Type = TokenRightParen
		tok.Value = ")"
		l.readChar()
	case ',':
		tok.Type = TokenComma
		tok.Value = ","
		l.readChar()
	case '.':
		tok.Type = TokenDot
		tok.Value = "."
		l.readChar()
	case '*':
		tok.Type = TokenAsterisk
		tok.Value = "*"
		l.readChar()
	case '"':
		tok = l.readString('"')
	case '\'':
		tok = l.readString('\'')
	case '?':
		tok = l.readQueryParamStart()
	case '&':
		tok = l.readFormFieldStart()
	case ' ', '\t':
		tok = l.readWhitespace()
	default:
		if isLetter(l.ch) {
			tok = l.readIdentifierOrKeyword()
		} else if isDigit(l.ch) || (l.ch == '-' && isDigit(l.peekChar())) {
			tok = l.readNumber()
		} else {
			tok.Type = TokenText
			tok.Value = string(l.ch)
			l.readChar()
		}
	}

	return tok
}

func (l *Lexer) readRequestSeparator() Token {
	line := l.line
	col := l.column
	l.readChar()
	l.readChar()
	l.readChar()
	l.skipWhitespaceInLine()
	name := l.readToEndOfLine()
	return Token{
		Type:   TokenRequestSeparator,
		Value:  strings.TrimSpace(name),
		Line:   line,
		Column: col,
	}
}

func (l *Lexer) readLineComment() Token {
	line := l.line
	col := l.column
	if l.ch == '/' && l.peekChar() == '/' {
		l.readChar()
	}
	l.readChar()
	comment := l.readToEndOfLine()
	return Token{
		Type:   TokenComment,
		Value:  strings.TrimSpace(comment),
		Line:   line,
		Column: col,
	}
}

func (l *Lexer) isAnnotationLine() bool {
	pos := l.pos
	for pos < len(l.input) && (l.input[pos] == '#' || l.input[pos] == ' ' || l.input[pos] == '\t') {
		if l.input[pos] == '@' {
			return true
		}
		pos++
		if pos < len(l.input) && l.input[pos] == '@' {
			return true
		}
	}
	return false
}

func (l *Lexer) readCommentAnnotation() Token {
	line := l.line
	col := l.column
	l.readChar()
	for l.ch == ' ' || l.ch == '\t' {
		l.readChar()
	}
	if l.ch != '@' {
		comment := l.readToEndOfLine()
		return Token{
			Type:   TokenComment,
			Value:  strings.TrimSpace(comment),
			Line:   line,
			Column: col,
		}
	}
	l.readChar()
	name := l.readIdentifier()

	l.skipWhitespaceInLine()
	value := strings.TrimSpace(l.readToEndOfLine())
	return Token{
		Type:    TokenAnnotation,
		Value:   name,
		Literal: value,
		Line:    line,
		Column:  col,
	}
}

func (l *Lexer) readAnnotationOrVariable() Token {
	line := l.line
	col := l.column
	l.readChar()
	name := l.readIdentifier()

	l.skipWhitespaceInLine()
	if l.ch == '=' {
		l.readChar()
		l.skipWhitespaceInLine()
		value := strings.TrimSpace(l.readToEndOfLine())
		return Token{
			Type:    TokenVariable,
			Value:   name,
			Literal: value,
			Line:    line,
			Column:  col,
		}
	}

	value := strings.TrimSpace(l.readToEndOfLine())
	return Token{
		Type:    TokenAnnotation,
		Value:   name,
		Literal: value,
		Line:    line,
		Column:  col,
	}
}

func (l *Lexer) readBlockStart() Token {
	line := l.line
	col := l.column
	l.readChar()
	l.readChar()
	l.readChar()
	blockType := strings.TrimSpace(l.readToEndOfLine())

	switch strings.ToLower(blockType) {
	case "capture":
		return Token{Type: TokenCaptureStart, Value: blockType, Line: line, Column: col}
	case "multipart":
		return Token{Type: TokenMultipartStart, Value: blockType, Line: line, Column: col}
	case "graphql":
		return Token{Type: TokenGraphQLStart, Value: blockType, Line: line, Column: col}
	case "variables":
		return Token{Type: TokenVariablesStart, Value: blockType, Line: line, Column: col}
	case "db":
		return Token{Type: TokenDBStart, Value: blockType, Line: line, Column: col}
	case "shell":
		return Token{Type: TokenShellStart, Value: blockType, Line: line, Column: col}
	default:
		return Token{Type: TokenAssertionStart, Value: blockType, Line: line, Column: col}
	}
}

func (l *Lexer) readBlockEnd() Token {
	line := l.line
	col := l.column
	l.readChar()
	l.readChar()
	l.readChar()
	return Token{Type: TokenAssertionEnd, Value: "<<<", Line: line, Column: col}
}

func (l *Lexer) readVariableRef() Token {
	line := l.line
	col := l.column
	l.readChar()
	l.readChar()
	var builder strings.Builder
	for l.ch != 0 && !(l.ch == '}' && l.peekChar() == '}') {
		builder.WriteByte(l.ch)
		l.readChar()
	}
	if l.ch == '}' {
		l.readChar()
		l.readChar()
	}
	return Token{
		Type:   TokenVariableRef,
		Value:  builder.String(),
		Line:   line,
		Column: col,
	}
}

func (l *Lexer) readString(quote byte) Token {
	line := l.line
	col := l.column
	l.readChar()
	var builder strings.Builder
	for l.ch != 0 && l.ch != quote {
		if l.ch == '\\' && l.peekChar() == quote {
			l.readChar()
		}
		builder.WriteByte(l.ch)
		l.readChar()
	}
	if l.ch == quote {
		l.readChar()
	}
	return Token{
		Type:    TokenString,
		Value:   builder.String(),
		Literal: builder.String(),
		Line:    line,
		Column:  col,
	}
}

func (l *Lexer) readNumber() Token {
	line := l.line
	col := l.column
	var builder strings.Builder
	if l.ch == '-' {
		builder.WriteByte(l.ch)
		l.readChar()
	}
	for isDigit(l.ch) {
		builder.WriteByte(l.ch)
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		builder.WriteByte(l.ch)
		l.readChar()
		for isDigit(l.ch) {
			builder.WriteByte(l.ch)
			l.readChar()
		}
	}
	return Token{
		Type:   TokenNumber,
		Value:  builder.String(),
		Line:   line,
		Column: col,
	}
}

func (l *Lexer) readIdentifierOrKeyword() Token {
	line := l.line
	col := l.column
	ident := l.readIdentifier()
	upper := strings.ToUpper(ident)

	if isHTTPMethod(upper) {
		return Token{Type: TokenMethod, Value: upper, Line: line, Column: col}
	}

	lower := strings.ToLower(ident)
	switch lower {
	case "expect":
		return Token{Type: TokenExpect, Value: ident, Line: line, Column: col}
	case "from":
		return Token{Type: TokenFrom, Value: ident, Line: line, Column: col}
	case "true", "false":
		return Token{Type: TokenBoolean, Value: lower, Literal: lower == "true", Line: line, Column: col}
	case "null":
		return Token{Type: TokenNull, Value: ident, Line: line, Column: col}
	case "contains", "startswith", "endswith", "matches", "exists", "length",
		"includes", "in", "type", "each", "schema":
		return Token{Type: TokenOperator, Value: lower, Line: line, Column: col}
	}

	return Token{Type: TokenIdentifier, Value: ident, Line: line, Column: col}
}

func (l *Lexer) readOperatorWord() Token {
	line := l.line
	col := l.column
	l.readChar()
	ident := l.readIdentifier()
	return Token{
		Type:   TokenOperator,
		Value:  "!" + strings.ToLower(ident),
		Line:   line,
		Column: col,
	}
}

func (l *Lexer) readQueryParamStart() Token {
	line := l.line
	col := l.column
	l.readChar()
	l.skipWhitespaceInLine()
	return Token{Type: TokenQueryParam, Value: "?", Line: line, Column: col}
}

func (l *Lexer) readFormFieldStart() Token {
	line := l.line
	col := l.column
	l.readChar()
	l.skipWhitespaceInLine()
	return Token{Type: TokenQueryParam, Value: "&", Line: line, Column: col}
}

func (l *Lexer) readWhitespace() Token {
	line := l.line
	col := l.column
	var builder strings.Builder
	for l.ch == ' ' || l.ch == '\t' {
		builder.WriteByte(l.ch)
		l.readChar()
	}
	return Token{
		Type:   TokenWhitespace,
		Value:  builder.String(),
		Line:   line,
		Column: col,
	}
}

func (l *Lexer) readIdentifier() string {
	var builder strings.Builder
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '-' {
		builder.WriteByte(l.ch)
		l.readChar()
	}
	return builder.String()
}

func (l *Lexer) readToEndOfLine() string {
	var builder strings.Builder
	for l.ch != 0 && l.ch != '\n' && l.ch != '\r' {
		builder.WriteByte(l.ch)
		l.readChar()
	}
	return builder.String()
}

func (l *Lexer) skipWhitespaceInLine() {
	for l.ch == ' ' || l.ch == '\t' {
		l.readChar()
	}
}

func (l *Lexer) ReadRawUntilBlockEnd() string {
	var builder strings.Builder
	for l.ch != 0 {
		if l.ch == '<' && l.peekChars(3) == "<<<" {
			break
		}
		builder.WriteByte(l.ch)
		l.readChar()
	}
	return strings.TrimSpace(builder.String())
}

func (l *Lexer) ReadRestOfLine() string {
	return strings.TrimSpace(l.readToEndOfLine())
}

func (l *Lexer) CurrentLine() int {
	return l.line
}

func (l *Lexer) CurrentColumn() int {
	return l.column
}

func isLetter(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT", "WS":
		return true
	}
	return false
}
