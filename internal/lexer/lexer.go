// Package lexer implements the hand-written scanner for the Craft DSL.
// S3: only actor-related tokens are defined here. Future slices add keywords
// as each grammar construct is introduced. Unknown tokens are returned as
// TokenError so the parser can emit a recoverable diagnostic.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType identifies the kind of a scanned token.
type TokenType int

const (
	TokenEOF     TokenType = iota // end of input
	TokenError                    // unrecognised character
	TokenIdent                    // identifier (names, also contextual keywords)
	TokenLBrace                   // {
	TokenRBrace                   // }
	TokenNewline                  // \n (whitespace-only lines are skipped; only records significant newlines)

	// Keywords (lex as identifiers and classify by value — contextual keyword strategy from Q3)
	TokenKwActor   // actor
	TokenKwActors  // actors
	TokenKwUser    // user
	TokenKwSystem  // system
	TokenKwService // service

	// Future keyword slots (other slices add their tokens before TokenSentinel)
	TokenSentinel // keep last
)

var keywords = map[string]TokenType{
	"actor":   TokenKwActor,
	"actors":  TokenKwActors,
	"user":    TokenKwUser,
	"system":  TokenKwSystem,
	"service": TokenKwService,
}

// Token is a scanned unit from the source.
type Token struct {
	Type    TokenType
	Value   string
	Line    int // 1-based
	Column  int // 1-based (byte offset within the line)
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%v, %q, %d:%d)", t.Type, t.Value, t.Line, t.Column)
}

// Lexer scans a string of Craft DSL source.
type Lexer struct {
	src    []rune
	pos    int // current position in src
	line   int // current 1-based line
	col    int // current 1-based column
}

// New creates a Lexer for the given source text.
func New(src string) *Lexer {
	return &Lexer{src: []rune(src), pos: 0, line: 1, col: 1}
}

// All scans all tokens until EOF, including EOF.
func (l *Lexer) All() []Token {
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens
}

// Next returns the next token, advancing the lexer.
func (l *Lexer) Next() Token {
	l.skipWhitespaceAndComments()

	if l.pos >= len(l.src) {
		return l.token(TokenEOF, "")
	}

	ch := l.src[l.pos]

	switch {
	case ch == '{':
		return l.consume(TokenLBrace)
	case ch == '}':
		return l.consume(TokenRBrace)
	case ch == '\n':
		// Newlines are generally not significant for Craft's grammar; skip.
		l.advance()
		return l.Next()
	case isIdentStart(ch):
		return l.scanIdent()
	default:
		tok := l.token(TokenError, string(ch))
		l.advance()
		return tok
	}
}

// skipWhitespaceAndComments advances past spaces, tabs, carriage returns, and
// single-line comments (// ...).
func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\r':
			l.advance()
		case ch == '\n':
			// Only skip blank newlines; let the Next() see significant ones.
			// For Craft's grammar all newlines are insignificant, so skip them all here.
			l.advance()
		case ch == '/' && l.peek(1) == '/':
			// Single-line comment: consume until newline.
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.advance()
			}
		default:
			return
		}
	}
}

func (l *Lexer) scanIdent() Token {
	start := l.pos
	startLine := l.line
	startCol := l.col
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.advance()
	}
	val := string(l.src[start:l.pos])
	tt := TokenIdent
	if kw, ok := keywords[strings.ToLower(val)]; ok {
		tt = kw
		// Keywords are case-insensitive; normalise to lowercase.
		val = strings.ToLower(val)
	}
	return Token{Type: tt, Value: val, Line: startLine, Column: startCol}
}

func (l *Lexer) consume(tt TokenType) Token {
	tok := l.token(tt, string(l.src[l.pos]))
	l.advance()
	return tok
}

func (l *Lexer) token(tt TokenType, val string) Token {
	return Token{Type: tt, Value: val, Line: l.line, Column: l.col}
}

func (l *Lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) peek(offset int) rune {
	idx := l.pos + offset
	if idx < len(l.src) {
		return l.src[idx]
	}
	return 0
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}
