package syntax_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// TestRoundTrip verifies that the green tree reassembles to the exact source text
// for every fixture in testdata/corpus/, walked RECURSIVELY.
//
// This test used to call os.ReadDir (non-recursive) on the corpus root, but
// all 55 corpus .craft files live in subdirectories (02_domains/,
// 06_exposures/, ...), so the loop iterated only top-level dir entries,
// skipped them all as non-.craft, and tested nothing — the round-trip
// guarantee was vacuously "green". filepath.WalkDir fixes that.
func TestRoundTrip(t *testing.T) {
	corpus := "../../testdata/corpus"
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("corpus not found: %v", err)
	}
	count := 0
	err := filepath.WalkDir(corpus, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".craft") {
			return nil
		}
		count++
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, _ := filepath.Rel(corpus, path)
		t.Run(rel, func(t *testing.T) {
			g, _, _ := syntax.Parse(string(src))
			got := reassembleGreen(g)
			if got != string(src) {
				t.Errorf("round-trip mismatch for %s\nwant: %q\ngot:  %q",
					rel, string(src), got)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", corpus, err)
	}
	if count == 0 {
		t.Fatalf("no .craft files found under %s — corpus walk is broken", corpus)
	}
	t.Logf("round-tripped %d corpus .craft files", count)
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
