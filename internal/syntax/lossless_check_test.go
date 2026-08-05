package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func reassemble2(g *green.GreenNode) string {
	var sb strings.Builder
	var walk func(*green.GreenNode)
	walk = func(n *green.GreenNode) {
		for _, c := range n.Children {
			switch v := c.(type) {
			case *green.GreenToken:
				sb.WriteString(v.Text)
			case *green.GreenNode:
				walk(v)
			}
		}
	}
	walk(g)
	return sb.String()
}

func checkLossless(t *testing.T, src string) {
	t.Helper()
	g, _, _ := syntax.Parse(src)
	got := reassemble2(g)
	if got != src {
		t.Errorf("NOT lossless\nwant: %q\ngot:  %q", src, got)
	}
}

func TestLossless_MultipleSpacesBetweenTokens(t *testing.T) {
	checkLossless(t, "actor   user   Alice")
}

func TestLossless_MultipleNewlinesBetweenEntries(t *testing.T) {
	checkLossless(t, "actor user Alice\n\n\nactor user Bob")
}

func TestLossless_TrailingNewline(t *testing.T) {
	checkLossless(t, "actor user Alice\n")
}

func TestLossless_TrailingNewlines(t *testing.T) {
	checkLossless(t, "actor user Alice\n\n\n")
}

func TestLossless_LeadingNewlines(t *testing.T) {
	checkLossless(t, "\n\nactor user Alice")
}

func TestLossless_MixedWhitespace(t *testing.T) {
	checkLossless(t, "\n\nactor user Alice\n\n\nactor system Bob\n")
}

func TestLossless_IndentedBlock(t *testing.T) {
	checkLossless(t, "actors {\n    user Alice\n    system Bob\n}\n")
}

func TestLossless_TabIndent(t *testing.T) {
	checkLossless(t, "actors {\n\tuser Alice\n}\n")
}

// TestLossless_QuotedString is the controller-verified repro for Bug 1: the
// green tree's reassembled text for a quoted-string token dropped the
// leading `"`, duplicated the last content character, and kept the closing
// `"` — e.g. `import "hello"\n` round-tripped as `import helloo"\n`.
func TestLossless_QuotedString(t *testing.T) {
	checkLossless(t, "import \"hello\"\n")
}

func TestLossless_QuotedString_ExactCorruptionRepro(t *testing.T) {
	src := "import \"hello\"\n"
	g, _, _ := syntax.Parse(src)
	got := reassemble2(g)
	corrupted := "import helloo\"\n"
	if got == corrupted {
		t.Fatalf("round-trip still produces the documented corruption %q", corrupted)
	}
	if got != src {
		t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
	}
}

// Non-ASCII sources. The lexer scans []rune, so Token.Column counts runes;
// feeding that column to LineIndex.Offset (which adds it to a byte line-start)
// used to under-compute every token start after a multi-byte character on the
// same line. The negative gap it produced was dropped by emitWhitespaceBefore
// while prevEnd still moved forward, so those bytes were counted twice and the
// green tree ended up wider than the source.
func TestLossless_NonASCIIInString(t *testing.T) {
	checkLossless(t, "use_case \"Registo de Utilizadores ção\" {\n  when User creates Account\n}\n")
}

func TestLossless_NonASCIIInComment(t *testing.T) {
	checkLossless(t, "// nota — aqui\nactors {\n\tuser Alice\n}\n")
}

func TestLossless_NonASCIIIdentifier(t *testing.T) {
	checkLossless(t, "actors {\n\tuser Ação\n\tsystem Bob\n}\n")
}

func TestLossless_AstralPlaneRune(t *testing.T) {
	checkLossless(t, "use_case \"ship it 🚀 now\" {\n  when User creates Account\n}\n")
}

// checkGreenWidth asserts the tree-width invariant: the root green node's width
// must equal len(src) exactly, so no red-tree offset can ever point past EOF.
// A violation is what crashed textDocument/semanticTokens/full: an out-of-range
// token offset reached LineIndex.UTF16Col and panicked on src[lineStart:offset].
func checkGreenWidth(t *testing.T, src string) {
	t.Helper()
	g, _, _ := syntax.Parse(src)
	root := syntax.Root(g)
	if got, want := int(root.TextRange().End), len(src); got != want {
		t.Errorf("green width %d != len(src) %d (overshoot %d)", got, want, got-want)
	}
	for _, tok := range root.AllTokens() {
		if end := int(tok.TextRange().End); end > len(src) {
			t.Errorf("token %v ends at %d, past len(src) %d", tok.Kind(), end, len(src))
		}
	}
}

func TestGreenWidth_MatchesSourceLength(t *testing.T) {
	cases := map[string]string{
		"ascii":            "actors {\n\tuser Alice\n}\n",
		"accent in string": "use_case \"Registo ção\" {\n  when User creates Account\n}\n",
		"accent in ident":  "actors {\n\tuser Ação\n\tsystem Bob\n}\n",
		"em dash comment":  "// nota — aqui\nactors {\n\tuser Alice\n}\n",
		"astral rune":      "use_case \"ship it 🚀 now\" {\n  when User creates Account\n}\n",
		"many multibyte":   "actors {\n\tuser Ação Café Núñez Über\n}\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) { checkGreenWidth(t, src) })
	}
}
