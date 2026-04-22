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
			name:      "comment skipped",
			src:       "// a comment\nactor user Foo",
			wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "underscore name",
			src:       "actor user My_Actor",
			wantTypes: []lexer.TokenType{lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
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
