package syntax

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lexer"
)

// parseRefText builds a Parser over src, calls parseRef() once at the start
// of the input, and returns the ref's raw text and leading kind word (if
// any). Used by tests that exercise the standalone parseRef helper without
// wiring it into any production parse path (that's Task 4+).
func parseRefText(t *testing.T, src string) (string, string) {
	t.Helper()
	l := lexer.New(src)
	p := &Parser{tokens: l.All(), src: src}
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

// Concatenating the green tree's tokens must reproduce src exactly: red-tree
// offsets are accumulated green widths, so any drift puts token offsets past
// EOF and every position the LSP derives from them is wrong. The invariant is
// cheap to check and should never fire on real input, since token text is a
// source slice by construction (Tasks 1-2); a violation can only be a parser
// bug, which is why checkTreeText panics under testing.Testing() rather than
// returning a diagnostic like the rest of this package's checks do.
func TestCheckTreeText(t *testing.T) {
	tok := func(text string) *green.GreenToken {
		return &green.GreenToken{Kind: SyntaxKindIdent, Text: text}
	}
	root := green.NewGreenNode(SyntaxKindFile, []green.GreenElement{tok("actor"), tok(" Foo")})

	if diags := checkTreeText(root, "actor Foo"); len(diags) != 0 {
		t.Errorf("tree text matches src: got %d diagnostics, want none: %+v", len(diags), diags)
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("mismatched tree text: expected a panic under testing.Testing(), got none")
			}
			msg, ok := r.(string)
			if !ok {
				t.Fatalf("panic value = %v (%T), want string", r, r)
			}
			if !strings.Contains(msg, "does not reproduce") {
				t.Errorf("panic message = %q, want it to describe a tree/source mismatch", msg)
			}
		}()
		checkTreeText(root, "actor Bar")
		t.Fatal("checkTreeText returned instead of panicking")
	}()

	if diags := checkTreeText(nil, "actor Foo"); len(diags) != 0 {
		t.Errorf("nil root: got %d diagnostics, want none", len(diags))
	}
}
