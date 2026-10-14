package lsp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// fuzzOutline renders the step tree of src: use cases, scenarios, block
// kinds, branch labels and action subjects. Formatting must not change it.
func fuzzOutline(src string) string {
	g, _, _ := syntax.Parse(src)
	var b strings.Builder
	var walk func(steps []syntax.StepDecl, depth int)
	walk = func(steps []syntax.StepDecl, depth int) {
		for _, st := range steps {
			ind := strings.Repeat("  ", depth)
			if a, ok := st.Action(); ok {
				b.WriteString(ind + "action " + a.SubjectName() + " | " + a.VerbValue() + "\n")
				continue
			}
			if n, ok := st.Note(); ok {
				b.WriteString(ind + "note " + string(n.Kind()) + " " + strconv.Quote(n.Text()) + "\n")
				continue
			}
			blk, _ := st.Block()
			b.WriteString(ind + string(blk.Kind()) + "\n")
			for _, br := range blk.Branches() {
				b.WriteString(ind + "  branch " + strconv.Quote(br.Label()) + "\n")
				walk(br.Steps(), depth+2)
			}
		}
	}
	for _, uc := range syntax.AsFile(syntax.Root(g)).UseCases() {
		b.WriteString("use_case " + uc.Name() + "\n")
		for _, sc := range uc.Scenarios() {
			b.WriteString(" scenario " + strconv.Quote(sc.Name()) + "\n")
			walk(sc.Steps(), 1)
		}
	}
	return b.String()
}

func fuzzFormatterBails(src string) bool {
	_, _, diags := syntax.Parse(src)
	for _, d := range diags {
		if bailsFormatting(d) {
			return true
		}
	}
	return false
}

// FuzzFormatBranches checks the formatter's invariants on arbitrary edits of
// the branch-block fixtures. Seeds run in a plain `go test`.
func FuzzFormatBranches(f *testing.F) {
	for _, name := range []string{"alt_else", "opt", "nested_blocks", "branch_tabs", "branches_full"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "corpus", "04_use_cases", name+".craft"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(raw))
	}
	f.Fuzz(func(t *testing.T, src string) {
		if fuzzFormatterBails(src) {
			t.Skip("input does not parse cleanly")
		}
		once := FormatDocument(src)
		if twice := FormatDocument(once); twice != once {
			t.Fatalf("not idempotent:\nonce:\n%s\ntwice:\n%s", once, twice)
		}
		if a, b := fuzzOutline(src), fuzzOutline(once); a != b {
			t.Fatalf("step outline changed:\nbefore:\n%s\nafter:\n%s\noutput:\n%s", a, b, once)
		}
		inComment := multilineCommentLines(once)
		for i, line := range strings.Split(once, "\n") {
			if inComment[i] {
				continue // text inside a block comment is kept verbatim
			}
			ws := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			if strings.Contains(ws, "\t") {
				t.Fatalf("tab in leading whitespace on line %d: %q\n%s", i+1, line, once)
			}
		}
	})
}

// multilineCommentLines marks the lines of src that continue a comment token
// begun on an earlier line; the formatter keeps those verbatim.
func multilineCommentLines(src string) map[int]bool {
	g, _, _ := syntax.Parse(src)
	out := map[int]bool{}
	for _, tok := range syntax.Root(g).AllTokens() {
		if !isCommentKind(tok.Kind()) {
			continue
		}
		line := strings.Count(src[:tok.Offset()], "\n")
		for k := 0; k < strings.Count(tok.Text(), "\n"); k++ {
			out[line+k+1] = true
		}
	}
	return out
}
