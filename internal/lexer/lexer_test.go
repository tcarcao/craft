package lexer_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/lexer"
)

func TestLexer_Actors(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantTypes []lexer.TokenType
	}{
		{
			name:      "individual actor",
			src:       "actor user Customer",
			wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "actors block",
			src:       "actors {\n  user Foo\n}",
			wantTypes: []lexer.TokenType{lexer.TokenKwActors, lexer.TokenLBrace, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenRBrace, lexer.TokenEOF},
		},
		{
			name:      "system keyword",
			src:       "actor system BG",
			wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwSystem, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "service keyword",
			src:       "actor service DB",
			wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwService, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "line comment returned as token",
			src:       "// a comment\nactor user Foo",
			wantTypes: []lexer.TokenType{lexer.TokenLineComment, lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "underscore name",
			src:       "actor user My_Actor",
			wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name: "line comment at EOF no newline",
			src:  "// comment at eof",
			wantTypes: []lexer.TokenType{
				lexer.TokenLineComment,
				lexer.TokenEOF,
			},
		},
		{
			name: "inline line comment after tokens",
			src:  "actor user Foo // inline comment",
			wantTypes: []lexer.TokenType{
				lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent,
				lexer.TokenLineComment,
				lexer.TokenEOF,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			toks := l.All()
			if len(toks) != len(tc.wantTypes) {
				t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
			}
			for i, tok := range toks {
				if tok.Type != tc.wantTypes[i] {
					t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
				}
			}
		})
	}
}

func TestLexer_BlockComment(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantTypes []lexer.TokenType
	}{
		{
			name:      "block comment returned as token",
			src:       "/* this is a comment */ actor user Foo",
			wantTypes: []lexer.TokenType{lexer.TokenBlockComment, lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "multi-line block comment",
			src:       "/* line1\nline2 */ domain Foo { }",
			wantTypes: []lexer.TokenType{lexer.TokenBlockComment, lexer.TokenKwDomain, lexer.TokenIdent, lexer.TokenLBrace, lexer.TokenRBrace, lexer.TokenEOF},
		},
		{
			name:      "unclosed block comment reaches EOF safely",
			src:       "/* unclosed",
			wantTypes: []lexer.TokenType{lexer.TokenBlockComment, lexer.TokenEOF},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			toks := l.All()
			if len(toks) != len(tc.wantTypes) {
				t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
			}
			for i, tok := range toks {
				if tok.Type != tc.wantTypes[i] {
					t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
				}
			}
		})
	}
}

func TestLexer_IdentifierWithDot(t *testing.T) {
	tests := []struct{ src, want string }{
		{"my.service", "my.service"},
		{"v1.2.3", "v1.2.3"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			l := lexer.New(tc.src)
			tok := l.Next()
			if tok.Type != lexer.TokenIdent {
				t.Fatalf("expected TokenIdent, got %v", tok.Type)
			}
			if tok.Value != tc.want {
				t.Errorf("got %q want %q", tok.Value, tc.want)
			}
		})
	}
}

func TestLexer_LineNumbers(t *testing.T) {
	src := "actors {\n    user Foo\n    system Bar\n}"
	l := lexer.New(src)
	toks := l.All()

	// user and Foo should be on line 2
	// system and Bar should be on line 3
	var userLine, fooLine, systemLine, barLine int
	for _, tok := range toks {
		switch tok.Value {
		case "user":
			userLine = tok.Line
		case "Foo":
			fooLine = tok.Line
		case "system":
			systemLine = tok.Line
		case "Bar":
			barLine = tok.Line
		}
	}

	if userLine != 2 {
		t.Errorf("user line: got %d want 2", userLine)
	}
	if fooLine != 2 {
		t.Errorf("Foo line: got %d want 2", fooLine)
	}
	if systemLine != 3 {
		t.Errorf("system line: got %d want 3", systemLine)
	}
	if barLine != 3 {
		t.Errorf("Bar line: got %d want 3", barLine)
	}
}

func TestLexer_StringEscapes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "escaped quote",           src: `"He said \"hi\""`, want: `He said "hi"`},
		{name: "escaped backslash",       src: `"path\\file"`,      want: `path\file`},
		{name: "escaped newline",         src: `"line1\nline2"`,    want: "line1\nline2"},
		{name: "escaped tab",             src: `"col1\tcol2"`,      want: "col1\tcol2"},
		{name: "escaped carriage return", src: `"cr\r"`,            want: "cr\r"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			tok := l.Next()
			if tok.Type != lexer.TokenString {
				t.Fatalf("expected TokenString, got %v", tok.Type)
			}
			if tok.Value != tc.want {
				t.Errorf("got %q want %q", tok.Value, tc.want)
			}
		})
	}
}

func TestLexer_NumberTokens(t *testing.T) {
	tests := []struct {
		src       string
		wantType  lexer.TokenType
		wantValue string
	}{
		{"42",    lexer.TokenNumber,     "42"},
		{"1.5",   lexer.TokenNumber,     "1.5"},
		{"90%",   lexer.TokenPercentage, "90%"},
		{"25.5%", lexer.TokenPercentage, "25.5%"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			l := lexer.New(tc.src)
			tok := l.Next()
			if tok.Type != tc.wantType {
				t.Errorf("type: got %v want %v", tok.Type, tc.wantType)
			}
			if tok.Value != tc.wantValue {
				t.Errorf("value: got %q want %q", tok.Value, tc.wantValue)
			}
		})
	}
}

// TestLexer_UnterminatedString verifies that scanString stops at '\n',
// stores the partial content (no quotes) in TokenError.Value, and leaves
// the token after the newline intact in the stream.
func TestLexer_UnterminatedString(t *testing.T) {
	src := "\"OrderPlaced\n}"
	l := lexer.New(src)
	tok := l.Next()
	if tok.Type != lexer.TokenError {
		t.Fatalf("expected TokenError for unterminated string, got %v", tok.Type)
	}
	if tok.Value != "OrderPlaced" {
		t.Errorf("partial content: got %q, want %q", tok.Value, "OrderPlaced")
	}
	if tok.Column != 1 {
		t.Errorf("column: got %d, want 1 (opening quote)", tok.Column)
	}
	// The `}` on the next line must still be in the stream.
	next := l.Next()
	if next.Type != lexer.TokenRBrace {
		t.Errorf("token after unterminated string: got %v, want TokenRBrace", next.Type)
	}
}

// TestLexer_UnterminatedString_BackslashBeforeNewline verifies that a trailing
// backslash before a newline also terminates the string rather than continuing.
func TestLexer_UnterminatedString_BackslashBeforeNewline(t *testing.T) {
	src := "\"Foo\\\n}"
	l := lexer.New(src)
	tok := l.Next()
	if tok.Type != lexer.TokenError {
		t.Fatalf("expected TokenError for backslash-at-EOL, got %v", tok.Type)
	}
	// Partial content is "Foo" (backslash not yet appended since we broke before consuming it).
	if tok.Value != "Foo" {
		t.Errorf("partial content: got %q, want %q", tok.Value, "Foo")
	}
	next := l.Next()
	if next.Type != lexer.TokenRBrace {
		t.Errorf("token after unterminated string: got %v, want TokenRBrace", next.Type)
	}
}

func TestLexer_PunctuationTokens(t *testing.T) {
	tests := []struct {
		src      string
		wantType lexer.TokenType
	}{
		{"(", lexer.TokenLParen},
		{")", lexer.TokenRParen},
		{"->", lexer.TokenArrow},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			l := lexer.New(tc.src)
			tok := l.Next()
			if tok.Type != tc.wantType {
				t.Errorf("got %v want %v", tok.Type, tc.wantType)
			}
		})
	}
}

func TestLexer_DocComment(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantTypes []lexer.TokenType
	}{
		{
			name:      "doc comment distinguished from line comment",
			src:       "/// doc\nactor user Alice",
			wantTypes: []lexer.TokenType{lexer.TokenDocComment, lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "regular line comment still works",
			src:       "// regular\nactor user Alice",
			wantTypes: []lexer.TokenType{lexer.TokenLineComment, lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "doc comment at EOF no newline",
			src:       "/// doc at eof",
			wantTypes: []lexer.TokenType{lexer.TokenDocComment, lexer.TokenEOF},
		},
		{
			name:      "doc comment value includes ///",
			src:       "/// hello world",
			wantTypes: []lexer.TokenType{lexer.TokenDocComment, lexer.TokenEOF},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			toks := l.All()
			if len(toks) != len(tc.wantTypes) {
				t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
			}
			for i, tok := range toks {
				if tok.Type != tc.wantTypes[i] {
					t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
				}
			}
		})
	}
	// Value check: doc comment value must start with ///
	l := lexer.New("/// my doc")
	toks := l.All()
	if toks[0].Type != lexer.TokenDocComment {
		t.Fatalf("expected TokenDocComment, got %v", toks[0].Type)
	}
	if len(toks[0].Value) < 3 || toks[0].Value[:3] != "///" {
		t.Errorf("doc comment value should start with ///, got %q", toks[0].Value)
	}
}

func TestLexer_ImportKeyword(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantTypes []lexer.TokenType
	}{
		{
			name:      "import keyword",
			src:       `import "other.craft"`,
			wantTypes: []lexer.TokenType{lexer.TokenKwImport, lexer.TokenString, lexer.TokenEOF},
		},
		{
			name:      "import not confused with identifier",
			src:       "import_service",
			wantTypes: []lexer.TokenType{lexer.TokenIdent, lexer.TokenEOF},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			toks := l.All()
			if len(toks) != len(tc.wantTypes) {
				t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
			}
			for i, tok := range toks {
				if tok.Type != tc.wantTypes[i] {
					t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
				}
			}
		})
	}
}
