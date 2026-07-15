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
