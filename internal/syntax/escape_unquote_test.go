package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

// TestEscapedString_ContentUnquotes_AndRoundTrips locks the two independent
// implementations of string-escape handling against silent desync:
// lexer.Lexer.scanString (internal/lexer/lexer.go) decides what counts as a
// valid escape and feeds the parser's raw token text, while
// unquoteStringText (internal/syntax/ast.go) reimplements the same escape
// switch to turn that raw text back into content. If the two ever drift,
// content accessors like UseCaseDecl.Name() would silently produce the wrong
// string while the raw round-trip stayed green (or vice versa) — this test
// checks both properties for the same escaped source in one place.
func TestEscapedString_ContentUnquotes_AndRoundTrips(t *testing.T) {
	// Raw Go string literal: backslash-quote sequences below are literal
	// source bytes, i.e. the .craft source text itself contains \" escapes.
	src := `use_case "She said \"hi\"" {
    when Customer submits PaymentForm
    PaymentService asks Bank to process
}`

	g, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", diags)
	}

	// Content path: Name() must resolve the escapes and drop the quotes.
	file := syntax.AsFile(syntax.Root(g))
	ucs := file.UseCases()
	if len(ucs) != 1 {
		t.Fatalf("expected 1 use_case, got %d", len(ucs))
	}
	const want = `She said "hi"`
	if got := ucs[0].Name(); got != want {
		t.Errorf("expected unquoted/unescaped name %q, got %q", want, got)
	}

	// Same content path via the exported StringAwareText wrapper used by
	// out-of-package (e.g. internal/lsp) content-read call sites.
	if title := ucs[0].Title(); title == nil {
		t.Fatal("expected non-nil title token")
	} else if got := syntax.StringAwareText(*title); got != want {
		t.Errorf("StringAwareText: expected %q, got %q", want, got)
	}

	// Raw path: the projection layer must still see the exact original
	// quoted+escaped text for round-trip purposes.
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	if len(doc.UseCases) != 1 || doc.UseCases[0].Name != want {
		t.Fatalf("expected projected UseCase.Name %q, got %+v", want, doc.UseCases)
	}

	// Round-trip path: the green tree must reassemble to byte-identical
	// source, preserving the \" escapes exactly as written.
	got := reassembleGreen(g)
	if got != src {
		t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
	}
}
