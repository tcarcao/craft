// Package lexer implements the hand-written scanner for the Craft DSL.
// S3: actor-related tokens. S4: adds domain/domains keywords.
// S5: adds service/services keywords, colon, comma, string literals for
// service names; contextual field keywords (contexts, data-stores, language)
// lex as plain identifiers per Q3.
// S6: adds use_case keyword. Contextual keywords (when, asks, notifies,
// listens, returns, to) remain plain identifiers per Q3.
// S7: adds arch keyword; TokenGT (>), TokenLBracket ([), TokenRBracket (]).
// presentation/gateway remain plain identifiers per Q3.
// S8: adds exposure keyword. Contextual field keywords (to, through, contexts)
// remain plain identifiers per Q3.
// Unknown tokens are returned as TokenError so the parser can emit a
// recoverable diagnostic.
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

	// S4: domain keywords
	TokenKwDomain  // domain
	TokenKwDomains // domains

	// S5: service keywords + punctuation
	TokenKwServices // services (block form)
	TokenColon      // :
	TokenComma      // ,
	TokenString     // "..." quoted string literal

	// S6: use_case keyword (contextual keywords when/asks/notifies/listens/returns/to are plain identifiers per Q3)
	TokenKwUseCase // use_case

	// S7: arch keyword + flow/modifier punctuation.
	// presentation/gateway remain plain identifiers per Q3.
	TokenKwArch    // arch
	TokenGT        // > (component flow operator)
	TokenLBracket  // [ (component modifier open)
	TokenRBracket  // ] (component modifier close)

	// S8: exposure keyword. Field keywords (to, through, contexts) are plain
	// identifiers per Q3.
	TokenKwExposure // exposure

	// Future keyword slots (other slices add their tokens before TokenSentinel)
	TokenSentinel // keep last
)

var keywords = map[string]TokenType{
	"actor":    TokenKwActor,
	"actors":   TokenKwActors,
	"user":     TokenKwUser,
	"system":   TokenKwSystem,
	"service":  TokenKwService,
	"domain":   TokenKwDomain,
	"domains":  TokenKwDomains,
	"services": TokenKwServices,
	"use_case": TokenKwUseCase,
	"arch":     TokenKwArch,
	"exposure": TokenKwExposure,
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
	case ch == ':':
		return l.consume(TokenColon)
	case ch == ',':
		return l.consume(TokenComma)
	case ch == '>':
		return l.consume(TokenGT)
	case ch == '[':
		return l.consume(TokenLBracket)
	case ch == ']':
		return l.consume(TokenRBracket)
	case ch == '"':
		return l.scanString()
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

// scanString scans a double-quoted string literal. Escape sequences are not
// interpreted (the raw text between the quotes is the token value). An
// unterminated string (no closing `"` before EOF) returns TokenError.
func (l *Lexer) scanString() Token {
	startLine := l.line
	startCol := l.col
	l.advance() // consume opening "
	var val []rune
	closed := false
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '"' {
			l.advance() // consume closing "
			closed = true
			break
		}
		if ch == '\\' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '"' {
			// escaped quote inside the string
			l.advance()
			val = append(val, '"')
			l.advance()
			continue
		}
		val = append(val, ch)
		l.advance()
	}
	if !closed {
		return Token{
			Type:   TokenError,
			Value:  "unterminated string literal",
			Line:   startLine,
			Column: startCol,
		}
	}
	return Token{Type: TokenString, Value: string(val), Line: startLine, Column: startCol}
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
		// Preserve original casing in the token value. The caller uses
		// the token Type to detect keywords; Value retains source spelling
		// so identifiers like "User" aren't lowercased when used as names.
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
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isIdentContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}
