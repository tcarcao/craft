package lsp

import (
	"strings"
	"testing"
)

func TestAlignAnnotations_AlignsAContiguousRun(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    Subscriptions asks Billing for a charge [POST /v1/charges]\n" +
		"    Billing asks Gateway to authorize [POST /pay/v2/authorize]\n" +
		"}\n"
	want := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    Subscriptions asks Billing for a charge  [POST /v1/charges]\n" +
		"    Billing asks Gateway to authorize        [POST /pay/v2/authorize]\n" +
		"}\n"
	if got := alignAnnotations(in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAlignAnnotations_IsIdempotent(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    LongerSubject asks B for c [GET /v1/y]\n" +
		"}\n"
	once := alignAnnotations(in)
	if twice := alignAnnotations(once); once != twice {
		t.Errorf("not idempotent\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestAlignAnnotations_NonAnnotatedLineDoesNotBreakTheRun(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    A asks C to do the thing\n" +
		"    A asks D for e [GET /v1/y]\n" +
		"}\n"
	// Both annotated lines have the same body width, so the shared column is
	// that width + 2 and each gets two spaces. The unannotated line is longer
	// than both and must not widen the column, nor split the run in two.
	got := alignAnnotations(in)
	if !strings.Contains(got, "for c  [POST /v1/x]") || !strings.Contains(got, "for e  [GET /v1/y]") {
		t.Errorf("run was broken by the unannotated line:\n%s", got)
	}
}

func TestAlignAnnotations_BlankLineResetsTheRun(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when A does x\n" +
		"    VeryLongSubjectHere asks B for c [POST /v1/x]\n" +
		"\n" +
		"  when B does y\n" +
		"    A asks C for d [GET /v1/y]\n" +
		"}\n"
	got := alignAnnotations(in)
	if !strings.Contains(got, "for d  [GET /v1/y]") {
		t.Errorf("second run should align independently:\n%s", got)
	}
}

func TestAlignAnnotations_LeavesTextWithoutAnnotationsAlone(t *testing.T) {
	in := "domain re {\n  Billing\n}\n"
	if got := alignAnnotations(in); got != in {
		t.Errorf("unannotated text changed:\ngot:  %q\nwant: %q", got, in)
	}
}

// TestAlignAnnotations_CommentLineTakesNoPartInTheColumn is the regression lock
// for a behaviour that writeAlignedActions had and this pass initially lost. A
// comment ending in `]` looked like an annotated line, and since a comment is
// usually the longest line in a scenario it pushed every real annotation out to
// match its width.
func TestAlignAnnotations_CommentLineTakesNoPartInTheColumn(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    // a very long explanatory note about [1]\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"}\n"
	got := alignAnnotations(in)
	if !strings.Contains(got, "    // a very long explanatory note about [1]\n") {
		t.Errorf("the comment line was itself rewritten:\n%s", got)
	}
	if !strings.Contains(got, "    A asks B for c  [POST /v1/x]") {
		t.Errorf("the comment's width leaked into the alignment column:\n%s", got)
	}
}

// TestAlignAnnotations_BracketInATrailingCommentIsNotAnAnnotation covers the
// same defect where the comment trails real content rather than owning the line.
func TestAlignAnnotations_BracketInATrailingCommentIsNotAnAnnotation(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c // see [1]\n" +
		"    LongerSubjectHere asks B for c [GET /v1/y]\n" +
		"}\n"
	got := alignAnnotations(in)
	if !strings.Contains(got, "    A asks B for c // see [1]\n") {
		t.Errorf("a bracket inside a trailing comment was aligned as an annotation:\n%s", got)
	}
}

// TestAlignAnnotations_PathInsideAnAnnotationStillAligns guards the rule above
// from over-reaching: the `//` in a URL sits after the `[`, so it must not
// disqualify a real annotation.
func TestAlignAnnotations_PathInsideAnAnnotationStillAligns(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [GET http://x/y]\n" +
		"    LongerSubject asks B for c [GET /v1/y]\n" +
		"}\n"
	got := alignAnnotations(in)
	var cols []int
	for _, line := range strings.Split(got, "\n") {
		if i := strings.Index(line, "["); i >= 0 {
			cols = append(cols, i)
		}
	}
	if len(cols) != 2 || cols[0] != cols[1] {
		t.Errorf("annotations should share a column, got %v:\n%s", cols, got)
	}
}
