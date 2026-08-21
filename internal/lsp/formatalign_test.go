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
	if got := alignAnnotations(in, nil, nil); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAlignAnnotations_IsIdempotent(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    LongerSubject asks B for c [GET /v1/y]\n" +
		"}\n"
	once := alignAnnotations(in, nil, nil)
	if twice := alignAnnotations(once, nil, nil); once != twice {
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
	got := alignAnnotations(in, nil, nil)
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
	got := alignAnnotations(in, nil, nil)
	if !strings.Contains(got, "for d  [GET /v1/y]") {
		t.Errorf("second run should align independently:\n%s", got)
	}
}

func TestAlignAnnotations_LeavesTextWithoutAnnotationsAlone(t *testing.T) {
	in := "domain re {\n  Billing\n}\n"
	if got := alignAnnotations(in, nil, nil); got != in {
		t.Errorf("unannotated text changed:\ngot:  %q\nwant: %q", got, in)
	}
}

// TestAlignAnnotations_CommentLineTakesNoPartInTheColumn is the regression lock
// for a behaviour that writeAlignedActions had and this pass initially lost. A
// comment ending in `]` looked like an annotated line, and since a comment is
// usually the longest line in a scenario it pushed every real annotation out to
// match its width.
func TestAlignAnnotations_CommentLineTakesNoPartInTheColumn(t *testing.T) {
	const note = "    // a very long explanatory note about [1]"
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		note + "\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"}\n"
	// A line comment runs to end of line, so the walker records its end as the
	// line's own byte length: every bracket on the line is comment text.
	got := alignAnnotations(in, nil, map[int]int{2: len(note)})
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
	const trailing = "    A asks B for c // see [1]"
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		trailing + "\n" +
		"    LongerSubjectHere asks B for c [GET /v1/y]\n" +
		"}\n"
	got := alignAnnotations(in, nil, map[int]int{2: len(trailing)})
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
	got := alignAnnotations(in, nil, nil)
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
	// Lines 2 and 3 are the lines the comment token runs off the end of; the
	// close on line 4 is where it stops, 13 bytes along.
	got := alignAnnotations(in, map[int]bool{2: true, 3: true}, map[int]int{4: 13})
	if !strings.Contains(got, "       thing [1]\n") {
		t.Errorf("an interior line was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "    A asks B for c  [POST /v1/x]") {
		t.Errorf("an interior line's width leaked into the column:\n%s", got)
	}
}

// TestSplitAnnotation_UrlInTheBodyDoesNotDisqualify covers what used to be an
// over-reach in a trailing-comment guard that no longer exists. The `//` in a
// URL is not a comment, the lexer knows that, and so no comment token ends on
// this line: the walker reports zero and the annotation stands. The old
// heuristic needed a special rule to reach the same answer, because it had only
// the text.
func TestSplitAnnotation_UrlInTheBodyDoesNotDisqualify(t *testing.T) {
	body, ann, ok := splitAnnotation("    A asks B for http://x/y [POST /v1/x]", 0)
	if !ok {
		t.Fatalf("a real annotation was disqualified by a URL in the body")
	}
	if body != "    A asks B for http://x/y" || ann != "[POST /v1/x]" {
		t.Errorf("got body %q ann %q", body, ann)
	}
}

// TestSplitAnnotation_TrailingCommentStillDisqualifies is the other half: a
// `//` that does open a comment still wins, because the token it opens runs to
// end of line and so every bracket on the line falls inside it.
func TestSplitAnnotation_TrailingCommentStillDisqualifies(t *testing.T) {
	for _, line := range []string{
		"    A asks B for c // see [1]",
		"    A asks B for c\t// see [1]",
		"    // see [1]",
		"    /// see [1]",
		// The bracket sits past the line's RUNE count but inside its byte
		// length. A rune-counted end would let this through as an annotation
		// and pad a comment out to a column.
		"    // ééééééééé [1]",
	} {
		if _, _, ok := splitAnnotation(line, len(line)); ok {
			t.Errorf("a bracket inside a comment was taken as an annotation: %q", line)
		}
	}
}

// TestSplitAnnotation_ContentAfterABlockCommentClose is the unit-level half of
// the block-comment-close alignment fix. Everything after a `*/` is ordinary
// content, and an action written there is a real action that has to align with
// its siblings.
//
// The `from` in each case is the offset just past that line's `*/`, which is
// what writeTokens records for it. The point of the table is that the spelling
// of the comment stops mattering: bare `*/`, a word before the close, a
// one-line `/* */`, a leading star, a bracket or a `//` in the comment text all
// reach the same answer, because all any of them affect is where the comment
// token happens to end and the walker measures that directly. Each of the last
// four used to need its own rule, and the word-led one had no rule at all.
func TestSplitAnnotation_ContentAfterABlockCommentClose(t *testing.T) {
	cases := []struct {
		line, body, ann string
		from            int
	}{
		{"     */ A asks B to b [POST /v1/b]", "     */ A asks B to b", "[POST /v1/b]", 7},
		{"       more */ A asks B to b [POST /v1/b]", "       more */ A asks B to b", "[POST /v1/b]", 14},
		{"    /* c */ A asks B to b [POST /v1/b]", "    /* c */ A asks B to b", "[POST /v1/b]", 11},
		{"     * see [1] */ A asks B to b [POST /v1/b]", "     * see [1] */ A asks B to b", "[POST /v1/b]", 17},
		{"     * // see */ A asks B to b [POST /v1/b]", "     * // see */ A asks B to b", "[POST /v1/b]", 16},
		// The defect this fix was for: a comment body line that closes after a
		// word, with a `//` in the text before the close. No pattern matched
		// it, so the old code left `from` at zero and then found that `//`.
		{"    see // here */ A asks B to c [GET /x]", "    see // here */ A asks B to c", "[GET /x]", 19},
	}
	for _, tc := range cases {
		body, ann, ok := splitAnnotation(tc.line, tc.from)
		if !ok {
			t.Errorf("content after a block-comment close was refused: %q", tc.line)
			continue
		}
		if body != tc.body || ann != tc.ann {
			t.Errorf("for %q got body %q ann %q, want body %q ann %q", tc.line, body, ann, tc.body, tc.ann)
		}
	}
}

// TestSplitAnnotation_TheCommentEndIsTheExactBoundary pins the gate itself
// rather than any spelling of a comment. A `[` at the reported end is the first
// byte of content and opens an annotation; one byte earlier it is the last byte
// of comment text and does not.
//
// This is the direction that corrupts. Too small a `from` does not merely lose
// an alignment, it lets the pass rewrite whitespace inside a comment, so the
// off-by-one below is the failure worth a test of its own.
//
// Lines a comment merely passes THROUGH have no comment ending on them, so this
// function is told zero and has nothing to say about them; they are excluded by
// the interior set instead, which
// TestAlignAnnotations_InteriorLinesTakeNoPartInThePass covers. The two guards
// used to be independent rules that each vetoed the other. There is one answer
// now and the walker gives it.
func TestSplitAnnotation_TheCommentEndIsTheExactBoundary(t *testing.T) {
	const line = "    /* note */ [GET /x]"
	open := strings.LastIndex(line, "[")

	if _, _, ok := splitAnnotation(line, open); !ok {
		t.Errorf("a `[` at the comment's end is content and must open an annotation: %q", line)
	}
	if _, _, ok := splitAnnotation(line, open+1); ok {
		t.Errorf("a `[` before the comment's end is comment text: %q", line)
	}
}

// TestSplitTrailingComment pins splitTrailingComment's contract on its own,
// the same way TestSplitAnnotation_* pins splitAnnotation's: start is handed
// down by the walker rather than worked out here, a comment-only line yields
// no cell (it is not sitting in the same column as one that follows content),
// and a start of zero means the walker recorded no trailing comment for the
// line at all.
func TestSplitTrailingComment(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		start    int
		wantBody string
		wantCell string
		wantOK   bool
	}{
		{"simple", "    kybc   // rollup: x", 11, "    kybc", "// rollup: x", true},
		{"no comment", "    kybc", 0, "", "", false},
		{"comment only line", "    // a note", 4, "", "", false},
		{"start zero means none", "    kybc // x", 0, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, cell, ok := splitTrailingComment(tt.line, tt.start)
			if ok != tt.wantOK || body != tt.wantBody || cell != tt.wantCell {
				t.Errorf("splitTrailingComment(%q, %d) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, tt.start, body, cell, ok, tt.wantBody, tt.wantCell, tt.wantOK)
			}
		})
	}
}
