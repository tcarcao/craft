package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestTokenAtOffset_Single(t *testing.T) {
	src := "actor user Foo"
	g, _, _ := syntax.Parse(src)
	root := syntax.Root(g)

	// offset 0 = 'a' in "actor"
	result := root.TokenAtOffset(0)
	single, ok := result.(syntax.Single)
	if !ok {
		t.Fatalf("expected Single, got %T", result)
	}
	if single.Token.Kind() != syntax.SyntaxKindKwActor {
		t.Errorf("token kind = %v, want SyntaxKindKwActor", single.Token.Kind())
	}
}

func TestTokenAtOffset_Between(t *testing.T) {
	src := "actor user Foo"
	g, _, _ := syntax.Parse(src)
	root := syntax.Root(g)

	// offset 5 = boundary between "actor"(0-4) and whitespace " "(5)
	result := root.TokenAtOffset(5)
	switch result.(type) {
	case syntax.Between, syntax.Single:
		// expected: either boundary or single whitespace token
	default:
		t.Errorf("expected Between or Single at offset 5, got %T", result)
	}
}

func TestTokenAtOffset_NoToken(t *testing.T) {
	src := "actor user Foo"
	g, _, _ := syntax.Parse(src)
	root := syntax.Root(g)

	// beyond end of file
	result := root.TokenAtOffset(9999)
	if _, ok := result.(syntax.NoToken); !ok {
		t.Errorf("expected NoToken beyond EOF, got %T", result)
	}
}

func TestNodeAt(t *testing.T) {
	src := "actor user Foo"
	g, _, _ := syntax.Parse(src)
	root := syntax.Root(g)

	node := root.NodeAt(0)
	if node == nil {
		t.Fatal("NodeAt(0) returned nil")
	}
	if node.Kind() != syntax.SyntaxKindActorDecl {
		t.Errorf("NodeAt(0) kind = %v, want SyntaxKindActorDecl", node.Kind())
	}
}

func TestTokenOffset_Correctness(t *testing.T) {
	// Verify that token offsets match the actual byte positions in the source.
	src := "actor user Foo"
	g, _, _ := syntax.Parse(src)
	root := syntax.Root(g)

	toks := root.AllTokens()
	reconstructed := ""
	for _, tok := range toks {
		start := int(tok.Offset())
		end := start + len(tok.Text())
		if end > len(src) {
			t.Errorf("token %v offset %d+%d out of bounds (src len %d)", tok.Kind(), start, len(tok.Text()), len(src))
			continue
		}
		if src[start:end] != tok.Text() {
			t.Errorf("token %v at offset %d: src[%d:%d]=%q, tok.Text()=%q",
				tok.Kind(), start, start, end, src[start:end], tok.Text())
		}
		reconstructed += tok.Text()
	}
	if reconstructed != src {
		t.Errorf("AllTokens reconstruction mismatch\nwant: %q\ngot:  %q", src, reconstructed)
	}
}
