package lsp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// fuzzOutline renders the step tree of src: use cases, scenarios, triggers,
// block kinds, branch labels, action subjects, notes, and surface operations.
// Formatting must not change it.
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
		for _, n := range uc.Notes() {
			b.WriteString(" note " + string(n.Kind()) + " " + strconv.Quote(n.Text()) + "\n")
		}
		for _, sc := range uc.Scenarios() {
			tr := sc.Trigger()
			b.WriteString(" scenario " + strconv.Quote(sc.Name()) + " " + string(tr.Kind()) + " " + tr.ContextName() + "\n")
			walk(sc.Steps(), 1)
		}
	}
	for _, sf := range syntax.AsFile(syntax.Root(g)).Surfaces() {
		b.WriteString("surface " + sf.Subject() + " callers " + strings.Join(sf.Callers(), ",") + "\n")
		for _, n := range sf.Notes() {
			b.WriteString(" note " + string(n.Kind()) + " " + strconv.Quote(n.Text()) + "\n")
		}
		for _, op := range sf.Operations() {
			b.WriteString(" " + string(op.Kind()) + " [" + op.OpText() + "]\n")
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
// the branch-block fixtures and the journey-semantics fixtures (par/and/loop,
// notes and findings, receives, the listens schedule tail, store hand-offs,
// surfaces). Seeds run in a plain `go test`.
func FuzzFormatBranches(f *testing.F) {
	for _, name := range []string{
		"04_use_cases/alt_else", "04_use_cases/opt", "04_use_cases/nested_blocks",
		"04_use_cases/branch_tabs", "04_use_cases/branches_full",
		"04_use_cases/par_fanout", "04_use_cases/par_and", "04_use_cases/loop",
		"04_use_cases/nested_par_loop_alt", "04_use_cases/notes_findings",
		"04_use_cases/receives_trigger", "04_use_cases/listens_schedule", "04_use_cases/store_handoff",
		"09_surfaces/surface_basic", "09_surfaces/surface_notes_callers_unknown",
	} {
		raw, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "corpus", filepath.FromSlash(name)+".craft"))
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
