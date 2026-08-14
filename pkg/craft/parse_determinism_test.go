package craft_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/pkg/craft"
)

// ParseFiles is the boundary that promises deterministic output, and this is the
// only test shape that can hold it to that: run the same workspace twice and
// compare. The bug it guards was invisible to every per-rule test, because the
// finding COUNT never changed — only which file and line each finding named, and
// the order they came back in.
//
// The workspaces below are deliberately multi-file with dedup keys shared across
// files, since that is the shape that makes the map-seed race observable.

// determinismWorkspace builds n files that all publish the same never-consumed,
// non-past-tense event and all declare an unused actor, so several dedup-keyed
// rules have n candidate files to blame and only one wins.
func determinismWorkspace(n int) map[string][]byte {
	files := make(map[string][]byte, n)
	for i := 0; i < n; i++ {
		src := fmt.Sprintf(`actor user Ghost%02d

domain d%02d {
  bc%02d
}

use_case "U%02d" {
  when Customer submits Payment
    bc%02d notifies "Order Place"
    bc%02d asks Nowhere%02d to do a thing
}`, i, i, i, i, i, i, i)
		files[fmt.Sprintf("f%02d.craft", i)] = []byte(src)
	}
	return files
}

// renderDiags serializes the diagnostic slice in the exact order ParseFiles
// returned it, so the comparison covers both the set of findings and their
// order — the two things that moved.
func renderDiags(diags []craft.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		fmt.Fprintf(&b, "%s\t%s\t%d:%d-%d:%d\t%s\t%s\n",
			d.SourceURI, d.Code,
			d.Range.Start.Line, d.Range.Start.Character,
			d.Range.End.Line, d.Range.End.Character,
			d.Severity, d.Message)
	}
	return b.String()
}

func TestParseFiles_DiagnosticsAreByteIdenticalAcrossRuns(t *testing.T) {
	files := determinismWorkspace(12)

	_, first, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("fixture produced no diagnostics, so it cannot detect instability")
	}
	want := renderDiags(first)

	for run := 1; run < 25; run++ {
		_, diags, err := craft.ParseFiles(files)
		if err != nil {
			t.Fatalf("run %d: ParseFiles: %v", run, err)
		}
		if got := renderDiags(diags); got != want {
			t.Fatalf("run %d differs from run 0.\n--- run 0 ---\n%s\n--- run %d ---\n%s", run, want, run, got)
		}
	}
}

// The count staying put while the content moves is precisely what let the bug
// survive, so assert the weaker property separately: if this passes while the
// test above fails, the failure is attribution, not a rule misfiring.
func TestParseFiles_DiagnosticCountIsStableEvenWhenAttributionIsNot(t *testing.T) {
	files := determinismWorkspace(12)

	_, first, _ := craft.ParseFiles(files)
	for run := 1; run < 25; run++ {
		_, diags, _ := craft.ParseFiles(files)
		if len(diags) != len(first) {
			t.Fatalf("run %d returned %d diagnostics, run 0 returned %d", run, len(diags), len(first))
		}
	}
}

// ParseFiles sorts the whole slice once at the end. Assert the resulting order
// directly: sorting each block on its own ordered diagnostics WITHIN a block but
// said nothing about the concatenation of blocks.
func TestParseFiles_DiagnosticsAreTotallyOrdered(t *testing.T) {
	_, diags, err := craft.ParseFiles(determinismWorkspace(12))
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	for i := 1; i < len(diags); i++ {
		if !diagLessOrEqual(diags[i-1], diags[i]) {
			t.Fatalf("diagnostics %d and %d are out of order:\n  %+v\n  %+v", i-1, i, diags[i-1], diags[i])
		}
	}
}

// diagLessOrEqual mirrors the documented sort key: file, line, character, code,
// then message. The message tiebreak is what makes the comparator a TOTAL order;
// without it two diagnostics sharing a position and a code compare equal and
// keep whatever order the upstream map iteration produced.
func diagLessOrEqual(a, b craft.Diagnostic) bool {
	if a.SourceURI != b.SourceURI {
		return a.SourceURI < b.SourceURI
	}
	if a.Range.Start.Line != b.Range.Start.Line {
		return a.Range.Start.Line < b.Range.Start.Line
	}
	if a.Range.Start.Character != b.Range.Start.Character {
		return a.Range.Start.Character < b.Range.Start.Character
	}
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	return a.Message <= b.Message
}

// Two diagnostics sharing a file, a position and a code — the exact case the
// comparator used to call "equal" — must come back in message order.
func TestParseFiles_SamePositionSameCodeIsOrderedByMessage(t *testing.T) {
	// Two services on one line both reference an undeclared context, producing
	// two unresolved-reference diagnostics anchored to the same service token.
	files := map[string][]byte{
		"a.craft": []byte(`domain Known {
  Real
}

services {
  Svc {
    contexts: Zulu, Alpha
  }
}`),
	}
	_, diags, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}

	var sameSpot []craft.Diagnostic
	for _, d := range diags {
		if d.Code == "craft/sema/unresolved-reference" {
			sameSpot = append(sameSpot, d)
		}
	}
	if len(sameSpot) < 2 {
		t.Fatalf("fixture yielded %d unresolved-reference diagnostics, need 2 to exercise the tiebreak", len(sameSpot))
	}

	// Guard against the assertion going vacuous: if the fixture ever stops
	// anchoring both diagnostics to the same token, the loop below would pass
	// without comparing anything.
	pairs := 0
	for i := 1; i < len(sameSpot); i++ {
		prev, cur := sameSpot[i-1], sameSpot[i]
		if prev.Range.Start != cur.Range.Start {
			continue
		}
		pairs++
		if prev.Message > cur.Message {
			t.Errorf("diagnostics at the same position are not message-ordered:\n  %q\n  %q", prev.Message, cur.Message)
		}
	}
	if pairs == 0 {
		t.Fatal("no two diagnostics shared a position, so the message tiebreak was never exercised")
	}
}
