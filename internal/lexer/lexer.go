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
	"unicode/utf8"
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
	TokenKwArch   // arch
	TokenGT       // > (component flow operator)
	TokenLBracket // [ (component modifier open)
	TokenRBracket // ] (component modifier close)

	// S8: exposure keyword. Field keywords (to, through, contexts) are plain
	// identifiers per Q3.
	TokenKwExposure // exposure

	// Q7: numeric values for modifier values and deployment rules
	TokenNumber     // [0-9]+ ('.' [0-9]+)?
	TokenPercentage // NUMBER '%'

	// deployment rule punctuation
	TokenLParen // (
	TokenRParen // )
	TokenArrow  // ->

	// Lossless syntax tree trivia tokens
	TokenLineComment  // // ... single-line comment (two slashes, not three)
	TokenBlockComment // /* ... */ block comment
	TokenDocComment   // /// ... doc comment (three slashes)

	// Top-level structural keywords
	TokenKwImport // import

	// context_map block keyword (Task 5). Edge keywords (the 8 DDD strategic
	// context-mapping patterns: customer_supplier/conformist/
	// anticorruption_layer/open_host_service/published_language/partnership/
	// shared_kernel/separate_ways) remain plain identifiers, matched by value
	// in the parser like asks/notifies (Q3).
	TokenKwContextMap // context_map

	// glossary block keyword (cross-context term-relation declarations).
	// Relation verbs (same_as/contrasts/distinct_from) remain plain
	// identifiers, matched by value in the parser like context_map's edge
	// verbs (Q3).
	TokenKwGlossary // glossary

	// Future keyword slots (other slices add their tokens before TokenSentinel)
	TokenSentinel // keep last
)

var keywords = map[string]TokenType{
	"actor":       TokenKwActor,
	"actors":      TokenKwActors,
	"user":        TokenKwUser,
	"system":      TokenKwSystem,
	"service":     TokenKwService,
	"domain":      TokenKwDomain,
	"domains":     TokenKwDomains,
	"services":    TokenKwServices,
	"use_case":    TokenKwUseCase,
	"arch":        TokenKwArch,
	"exposure":    TokenKwExposure,
	"import":      TokenKwImport,
	"context_map": TokenKwContextMap,
	"glossary":    TokenKwGlossary,
}

// Token is a scanned unit from the source.
type Token struct {
	Type  TokenType
	Value string
	Line  int // 1-based
	// Column is 1-based and counts RUNES within the line, not bytes — the
	// lexer scans []rune. Use it for line-relative comparisons (adjacency,
	// same-line checks) and for diagnostic positions. Never add it to a byte
	// line-start: on a line with multi-byte characters that under-computes the
	// byte position. Use Offset for byte positions.
	Column int
	// Offset is the 0-based BYTE offset of the token's first character from
	// the start of the source. This is the authoritative position for building
	// the green tree, whose widths must sum to len(src) exactly.
	Offset int
	// End is the 0-based BYTE offset one past the token's last byte, so that
	// src[Offset:End] is the token's verbatim source text for every token
	// kind, including malformed ones. The green tree slices this rather than
	// deriving length from token text: deriving it meant a wrong text produced
	// a consistently wrong length, which no width check could detect.
	End int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%v, %q, %d:%d)", t.Type, t.Value, t.Line, t.Column)
}

// Lexer scans a string of Craft DSL source.
type Lexer struct {
	src []rune
	// byteOffsets[i] is the byte offset in the original source of src[i].
	// It has len(src)+1 entries; the last one is len(source), so an EOF token
	// at pos == len(src) still has a valid offset.
	byteOffsets []int
	pos         int  // current position in src
	line        int  // current 1-based line
	col         int  // current 1-based column (in runes — see Token.Column)
	prevRune    rune // rune just consumed by advance(); 0 at start of input
}

// New creates a Lexer for the given source text.
func New(src string) *Lexer {
	// Ranging over the string yields the same rune sequence as []rune(src)
	// while also handing us each rune's true byte offset, including for
	// invalid UTF-8 (where both produce RuneError one byte at a time).
	runes := make([]rune, 0, len(src))
	offsets := make([]int, 0, len(src)+1)
	for i, r := range src {
		runes = append(runes, r)
		offsets = append(offsets, i)
	}
	offsets = append(offsets, len(src))
	return &Lexer{src: runes, byteOffsets: offsets, pos: 0, line: 1, col: 1}
}

// offsetAt returns the byte offset of the rune at index pos. pos may equal
// len(src), which yields the byte length of the whole source.
func (l *Lexer) offsetAt(pos int) int { return l.byteOffsets[pos] }

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
	l.skipWhitespace()

	if l.pos >= len(l.src) {
		return l.token(TokenEOF, "")
	}

	ch := l.src[l.pos]

	// A comment may only begin when the '/' is at the start of the input or
	// immediately preceded by whitespace. This keeps slashes inside prose
	// (URLs like http://api, ratios like 50/50) from being mis-lexed as
	// comments — a non-whitespace-preceded '/' falls through to the default
	// case below and is scanned as TokenError, which the parser sweeps into
	// prose text.
	precededByWS := l.pos == 0 ||
		l.prevRune == ' ' || l.prevRune == '\t' || l.prevRune == '\n' || l.prevRune == '\r'

	switch {
	case precededByWS && ch == '/' && l.peek(1) == '/' && l.peek(2) == '/':
		return l.scanDocComment()
	case precededByWS && ch == '/' && l.peek(1) == '/':
		return l.scanLineComment()
	case precededByWS && ch == '/' && l.peek(1) == '*':
		return l.scanBlockComment()
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
	case ch == '(':
		return l.consume(TokenLParen)
	case ch == ')':
		return l.consume(TokenRParen)
	case ch == '-' && l.peek(1) == '>':
		tok := l.token(TokenArrow, "->")
		l.advance()
		l.advance()
		return tok
	case unicode.IsDigit(ch):
		return l.scanNumber()
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

// skipWhitespace advances past spaces, tabs, carriage returns, and newlines.
func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) {
		switch l.src[l.pos] {
		case ' ', '\t', '\r', '\n':
			l.advance()
		default:
			return
		}
	}
}

// scanLineComment scans a // single-line comment and returns it as a token.
func (l *Lexer) scanLineComment() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	l.advance() // /
	l.advance() // /
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	return Token{Type: TokenLineComment, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
}

// scanDocComment scans a /// doc comment and returns it as a TokenDocComment.
func (l *Lexer) scanDocComment() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	l.advance() // /
	l.advance() // /
	l.advance() // /
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	return Token{Type: TokenDocComment, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
}

// scanBlockComment scans a /* ... */ block comment and returns it as a token.
// If EOF is reached without finding */, returns a TokenBlockComment whose Value
// does not end with */. Callers that need to distinguish malformed block comments
// must inspect the value.
func (l *Lexer) scanBlockComment() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	l.advance() // /
	l.advance() // *
	for l.pos < len(l.src) {
		if l.src[l.pos] == '*' && l.peek(1) == '/' {
			l.advance() // *
			l.advance() // /
			break
		}
		l.advance()
	}
	return Token{Type: TokenBlockComment, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
}

// scanString scans a double-quoted string literal. Supported escape sequences:
// \", \\, \n, \t, \r. Unknown escapes are passed through as-is (backslash + char).
// An unterminated string (no closing `"` before EOF) returns TokenError.
func (l *Lexer) scanString() Token {
	startLine := l.line
	startCol := l.col
	rawStart := l.pos // position of the opening `"`, for Offset/End
	l.advance()       // consume opening "
	var val []rune
	closed := false
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '"' {
			l.advance()
			closed = true
			break
		}
		if ch == '\n' {
			// Craft strings are single-line; don't advance past the newline so
			// the parser's token stream is not disrupted.
			break
		}
		if ch == '\\' && l.pos+1 < len(l.src) {
			if l.src[l.pos+1] == '\n' {
				// Trailing backslash before newline — consume the backslash and
				// treat as unterminated; '\n' is left for skipWhitespace.
				l.advance()
				break
			}
			l.advance() // consume backslash
			next := l.src[l.pos]
			l.advance() // consume escape char
			switch next {
			case '"':
				val = append(val, '"')
			case '\\':
				val = append(val, '\\')
			case 'n':
				val = append(val, '\n')
			case 't':
				val = append(val, '\t')
			case 'r':
				val = append(val, '\r')
			default:
				val = append(val, '\\', next)
			}
			continue
		}
		val = append(val, ch)
		l.advance()
	}
	if !closed {
		// val contains partial content up to the newline (no quotes).
		// Callers that compute a Range must add +1 for the opening `"`.
		return Token{Type: TokenError, Value: string(val), Line: startLine, Column: startCol, Offset: l.offsetAt(rawStart), End: l.offsetAt(l.pos)}
	}
	return Token{Type: TokenString, Value: string(val), Line: startLine, Column: startCol, Offset: l.offsetAt(rawStart), End: l.offsetAt(l.pos)}
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
	return Token{Type: tt, Value: val, Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
}

func (l *Lexer) scanNumber() Token {
	startLine := l.line
	startCol := l.col
	start := l.pos
	for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
		l.advance()
	}
	// Check for decimal part (only if followed by more digits, e.g. 1.5 not 1.identifier)
	if l.pos < len(l.src) && l.src[l.pos] == '.' &&
		l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1]) {
		l.advance() // consume '.'
		for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
			l.advance()
		}
	}
	// If followed by '%', emit as percentage (e.g. 90%)
	if l.pos < len(l.src) && l.src[l.pos] == '%' {
		l.advance()
		return Token{Type: TokenPercentage, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
	}
	// If followed by a letter/underscore (e.g. 30s, 1ms), scan as identifier for
	// backward compatibility with alphanumeric modifier values.
	if l.pos < len(l.src) && (unicode.IsLetter(l.src[l.pos]) || l.src[l.pos] == '_') {
		for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
			l.advance()
		}
		return Token{Type: TokenIdent, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
	}
	return Token{Type: TokenNumber, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol, Offset: l.offsetAt(start), End: l.offsetAt(l.pos)}
}

func (l *Lexer) consume(tt TokenType) Token {
	tok := l.token(tt, string(l.src[l.pos]))
	l.advance()
	return tok
}

// token builds a Token starting at the current position, before any
// advance() calls that consume it. Every caller of token() goes on to call
// advance() exactly once per rune already present in val (consume() advances
// once for its single-rune val; the "->" arrow site advances twice for its
// two-rune val; the zero-rune EOF val advances zero times), so the token's
// end is always l.pos plus that many runes, converted to a byte offset
// before those advances happen.
func (l *Lexer) token(tt TokenType, val string) Token {
	end := l.offsetAt(l.pos + utf8.RuneCountInString(val))
	return Token{Type: tt, Value: val, Line: l.line, Column: l.col, Offset: l.offsetAt(l.pos), End: end}
}

func (l *Lexer) advance() {
	if l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
		l.prevRune = ch
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
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}
