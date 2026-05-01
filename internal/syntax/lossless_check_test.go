package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/syntax"
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
