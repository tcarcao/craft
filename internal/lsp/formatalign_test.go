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
	if got := alignAnnotations(in, nil); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAlignAnnotations_IsIdempotent(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    LongerSubject asks B for c [GET /v1/y]\n" +
		"}\n"
	once := alignAnnotations(in, nil)
	if twice := alignAnnotations(once, nil); once != twice {
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
	got := alignAnnotations(in, nil)
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
	got := alignAnnotations(in, nil)
	if !strings.Contains(got, "for d  [GET /v1/y]") {
		t.Errorf("second run should align independently:\n%s", got)
	}
}

func TestAlignAnnotations_LeavesTextWithoutAnnotationsAlone(t *testing.T) {
	in := "domain re {\n  Billing\n}\n"
	if got := alignAnnotations(in, nil); got != in {
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
	got := alignAnnotations(in, nil)
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
	got := alignAnnotations(in, nil)
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
	got := alignAnnotations(in, nil)
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

// TestAlignAnnotations_InteriorLinesTakeNoPartInThePass pins the contract
// between the walker and this pass at the pass's own level. Lines the walker
// reports as interior to a token must neither be rewritten nor contribute their
// width to the column, however much they look like an annotated action, because
// nothing about a single line of text reveals that it came from the middle of
// somebody else's token.
func TestAlignAnnotations_InteriorLinesTakeNoPartInThePass(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    /* note\n" +
		"       thing [1]\n" +
		"       end */\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"}\n"
	// Lines 3 and 4 are the block comment's continuation lines.
	got := alignAnnotations(in, map[int]bool{3: true, 4: true})
	if !strings.Contains(got, "       thing [1]\n") {
		t.Errorf("an interior line was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "    A asks B for c  [POST /v1/x]") {
		t.Errorf("an interior line's width leaked into the column:\n%s", got)
	}
}

// TestSplitAnnotation_UrlInTheBodyDoesNotDisqualify covers the over-reach in
// the trailing-comment guard. The `//` in a URL follows a `:` rather than
// whitespace, so it does not open a comment and must not cost the line its
// alignment.
func TestSplitAnnotation_UrlInTheBodyDoesNotDisqualify(t *testing.T) {
	body, ann, ok := splitAnnotation("    A asks B for http://x/y [POST /v1/x]")
	if !ok {
		t.Fatalf("a real annotation was disqualified by a URL in the body")
	}
	if body != "    A asks B for http://x/y" || ann != "[POST /v1/x]" {
		t.Errorf("got body %q ann %q", body, ann)
	}
}

// TestSplitAnnotation_TrailingCommentStillDisqualifies is the other half: a
// `//` that does open a comment still wins.
func TestSplitAnnotation_TrailingCommentStillDisqualifies(t *testing.T) {
	for _, line := range []string{
		"    A asks B for c // see [1]",
		"    A asks B for c\t// see [1]",
		"    // see [1]",
	} {
		if _, _, ok := splitAnnotation(line); ok {
			t.Errorf("a bracket inside a comment was taken as an annotation: %q", line)
		}
	}
}
