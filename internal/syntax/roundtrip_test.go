package syntax_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/syntax"
)

// TestRoundTrip verifies that the green tree reassembles to the exact source text
// for every fixture in testdata/corpus/.
func TestRoundTrip(t *testing.T) {
	corpus := "../../testdata/corpus"
	entries, err := os.ReadDir(corpus)
	if err != nil {
		t.Skipf("corpus not found: %v", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".craft") {
			continue
		}
		path := filepath.Join(corpus, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		t.Run(e.Name(), func(t *testing.T) {
			g, _, _ := syntax.Parse(string(src))
			got := reassembleGreen(g)
			if got != string(src) {
				t.Errorf("round-trip mismatch for %s\nwant: %q\ngot:  %q",
					e.Name(), string(src), got)
			}
		})
	}
}

func reassembleGreen(g *green.GreenNode) string {
	var sb strings.Builder
	collectGreenText(g, &sb)
	return sb.String()
}

func collectGreenText(g *green.GreenNode, sb *strings.Builder) {
	for _, c := range g.Children {
		switch v := c.(type) {
		case *green.GreenToken:
			sb.WriteString(v.Text)
		case *green.GreenNode:
			collectGreenText(v, sb)
		}
	}
}
