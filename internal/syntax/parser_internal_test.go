package syntax

import (
	"testing"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/lexer"
)

// parseRefText builds a Parser over src, calls parseRef() once at the start
// of the input, and returns the ref's raw text and leading kind word (if
// any). Used by tests that exercise the standalone parseRef helper without
// wiring it into any production parse path (that's Task 4+).
func parseRefText(t *testing.T, src string) (string, string) {
	t.Helper()
	li := green.NewLineIndex(src)
	l := lexer.New(src)
	p := &Parser{tokens: l.All(), src: src, li: li}
	p.builder.StartNode(SyntaxKindFile)
	kind := p.parseRef()
	p.builder.FinishNode()
	root := p.builder.Finish()

	tree := Root(root)
	refs := tree.ChildNodes(SyntaxKindRef)
	if len(refs) != 1 {
		t.Fatalf("parseRefText(%q): expected 1 SyntaxKindRef node, got %d", src, len(refs))
	}
	return RefDecl{node: refs[0]}.RefText(), kind
}

func TestParseRef_Shapes(t *testing.T) {
	cases := []struct{ src, wantText, wantKind string }{
		{"vas.VasApplied", "vas.VasApplied", ""},
		{"com.olx.re.subscriptions.SubscriptionCreated", "com.olx.re.subscriptions.SubscriptionCreated", ""},
		{"bc:re/subscriptions", "bc:re/subscriptions", "bc"},
		{"term:billing/dunning", "term:billing/dunning", "term"},
		{"service:subscriptions-api", "service:subscriptions-api", "service"},
		// Regression: Column is rune-based but len(a.Value) is a byte count,
		// so adjacentTokens must compare rune lengths. Otherwise a multibyte
		// character in a segment (e.g. "café") silently truncates the ref.
		{"bc:café/billing", "bc:café/billing", "bc"},
		// Regression: a slug segment that lexes as a hard keyword (e.g. a BC
		// literally named "user"/"service"/"domain") must not truncate the
		// ref mid-slug. The continuation loop must accept
		// isAnyKeywordAsIdent tokens the same way the leading kind-word
		// branch already does.
		{"term:user/account", "term:user/account", "term"},
		{"bc:re/service", "bc:re/service", "bc"},
	}
	for _, c := range cases {
		txt, kind := parseRefText(t, c.src) // test helper wrapping a Parser over c.src
		if txt != c.wantText || kind != c.wantKind {
			t.Fatalf("%q -> (%q,%q), want (%q,%q)", c.src, txt, kind, c.wantText, c.wantKind)
		}
	}
}
