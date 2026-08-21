package lsp

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/fmtconfig"
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
	if got := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults()); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAlignAnnotations_IsIdempotent(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    LongerSubject asks B for c [GET /v1/y]\n" +
		"}\n"
	once := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
	if twice := alignCells(once, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults()); once != twice {
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
	got := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
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
	got := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
	if !strings.Contains(got, "for d  [GET /v1/y]") {
		t.Errorf("second run should align independently:\n%s", got)
	}
}

func TestAlignAnnotations_LeavesTextWithoutAnnotationsAlone(t *testing.T) {
	in := "domain re {\n  Billing\n}\n"
	if got := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults()); got != in {
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
	got := alignCells(in, nil, map[int]int{2: len(note)}, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
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
	got := alignCells(in, nil, map[int]int{2: len(trailing)}, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
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
	got := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
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
	got := alignCells(in, map[int]bool{2: true, 3: true}, map[int]int{4: 13}, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
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

// TestSplitTrailingComment_TrailingWhitespaceInsideTheCommentIsStripped pins
// a new, deliberate behaviour: splitTrailingComment TrimRights the comment
// cell itself, so trailing whitespace inside a line-comment TOKEN's own tail
// is dropped. This is end-of-line whitespace in the token's tail, not
// interior text -- the shipped annotation path already trimmed the same way
// (splitAnnotation's `trimmed := strings.TrimRight(line, " \t")`) -- but
// nothing pinned it for the comment cell before this test.
func TestSplitTrailingComment_TrailingWhitespaceInsideTheCommentIsStripped(t *testing.T) {
	const line = "    kybc   // rollup: x   "
	start := strings.Index(line, "//")
	_, cell, ok := splitTrailingComment(line, start)
	if !ok {
		t.Fatalf("expected a trailing comment cell")
	}
	if cell != "// rollup: x" {
		t.Errorf("trailing whitespace inside the comment's own tail should be stripped: got %q", cell)
	}
}

// TestAlignCellsScopes exercises all five scopes against one fixture whose
// three joins each isolate a different scope boundary:
//
//   - aaaaaaaaaa/bb are separated by a cell-less content line, which
//     discriminates strict (ends the run: neither aligns) from block (passes
//     through: they share a column).
//   - bb/cc are separated only by a `}`/`{` pair, no blank line, which
//     discriminates block (a brace ends the run) from file (a brace does not,
//     so cc joins the aaaaaaaaaa/bb column too).
//   - cc/d are separated by a blank line, which discriminates file (a blank
//     line ends even a file-scoped run) from decl (nothing ends a run, since
//     alignCells is already invoked once per top-level declaration, so d
//     joins every earlier line's column as well).
//
// The five scopes are ordered least to most aligning — off < strict < block
// < file < decl — and this fixture pins that each step aligns one more join
// than the last, which is the exact property finding 1's ruling restored:
// decl and file must diverge on the blank-line join, not agree.
func TestAlignCellsScopes(t *testing.T) {
	in := "domains {\n" +
		"    account {\n" +
		"        aaaaaaaaaa // one\n" +
		"        skip this line\n" +
		"        bb // two\n" +
		"    }\n" +
		"    ad {\n" +
		"        cc // three\n" +
		"    }\n" +
		"\n" +
		"    xy {\n" +
		"        d // four\n" +
		"    }\n" +
		"}"

	tests := []struct {
		scope  fmtconfig.Scope
		wantBB string
		wantCC string
		wantD  string
	}{
		{fmtconfig.ScopeOff, "        bb // two", "        cc // three", "        d // four"},
		{fmtconfig.ScopeStrict, "        bb // two", "        cc // three", "        d // four"},
		{fmtconfig.ScopeBlock, "        bb         // two", "        cc // three", "        d // four"},
		{fmtconfig.ScopeFile, "        bb         // two", "        cc         // three", "        d // four"},
		{fmtconfig.ScopeDecl, "        bb         // two", "        cc         // three", "        d          // four"},
	}
	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			hints := hintsFor(in)
			got := alignCells(in, nil, hints, splitTrailingComment, tt.scope, trailingCommentMinGap, fmtconfig.Defaults())
			if l := lineContaining(t, got, "two"); l != tt.wantBB {
				t.Errorf("scope %s: bb line got %q, want %q", tt.scope, l, tt.wantBB)
			}
			if l := lineContaining(t, got, "three"); l != tt.wantCC {
				t.Errorf("scope %s: cc line got %q, want %q", tt.scope, l, tt.wantCC)
			}
			if l := lineContaining(t, got, "four"); l != tt.wantD {
				t.Errorf("scope %s: d line got %q, want %q", tt.scope, l, tt.wantD)
			}
		})
	}
}

// TestCellPrecedence_BothCellsJoinsOnlyTheCommentColumn pins a documented
// limitation rather than a requirement. splitAnnotation requires a line to
// end in `]`, so it cannot see an annotation once a trailing comment follows
// it on the same line. Such a line therefore joins the comment column only,
// and keeps whatever spacing writeTokens gave the annotation (a single,
// unaligned space) rather than sharing a column with sibling annotations.
//
// This is deliberate, not an oversight: there are zero lines in the repo
// corpus carrying both cells, so closing the gap (bounding splitAnnotation's
// search at the comment start, or replacing the two passes with a single
// two-column aligner) is not worth building for a shape nothing produces. See
// docs/decisions/formatting-configuration.md, "Cell precedence, and a
// documented limitation". If this test ever needs to change because such
// lines start appearing, that change should be deliberate, not silent drift.
func TestCellPrecedence_BothCellsJoinsOnlyTheCommentColumn(t *testing.T) {
	src := "use_case \"X\" {\n" +
		"    when A does b\n" +
		"        A asks B to ccccc  [POST /v1/x]  // note\n" +
		"        A asks B to d  [GET /y]\n" +
		"        A asks B to eeeeeeee  // only a comment\n" +
		"        A asks B to f\n" +
		"}\n"
	got := FormatDocument(src)

	both := lineContaining(t, got, "ccccc")
	annOnly := lineContaining(t, got, "to d")
	commentOnly := lineContaining(t, got, "eeeeeeee")

	// The comment column is shared between the dual-cell line and the
	// comment-only line: the trailing-comment pass still sees and aligns it.
	bothCommentCol := strings.Index(both, "//")
	commentOnlyCol := strings.Index(commentOnly, "//")
	if bothCommentCol < 0 || commentOnlyCol < 0 || bothCommentCol != commentOnlyCol {
		t.Errorf("comment column not shared between the dual-cell line and the comment-only line:\ndual-cell:    %q\ncomment-only: %q", both, commentOnly)
	}

	// The annotation column is NOT shared with the sibling annotation-only
	// line: splitAnnotation never saw the dual-cell line's annotation, so it
	// keeps the single unaligned space writeTokens gave it rather than being
	// padded out to match "to d  [GET /y]".
	bothAnnCol := strings.Index(both, "[")
	annOnlyCol := strings.Index(annOnly, "[")
	if bothAnnCol < 0 || annOnlyCol < 0 {
		t.Fatalf("expected both lines to carry an annotation:\ndual-cell: %q\nann-only:  %q", both, annOnly)
	}
	if bothAnnCol == annOnlyCol {
		t.Errorf("annotation column unexpectedly shared with the dual-cell line; the documented limitation says it should not be:\ndual-cell: %q\nann-only:  %q", both, annOnly)
	}
	if want := strings.Index(both, "ccccc") + len("ccccc") + 1; bothAnnCol != want {
		t.Errorf("dual-cell line's annotation spacing was touched: got column %d (%q), want the single unaligned space at %d", bothAnnCol, both, want)
	}
}

func TestAlignCellsAlignsWithinABlock(t *testing.T) {
	in := "domains {\n" +
		"    account {\n" +
		"        aaaaaaaaaa // one\n" +
		"        bb // two\n" +
		"    }\n" +
		"}"
	got := alignCells(in, nil, hintsFor(in), splitTrailingComment, fmtconfig.ScopeBlock, trailingCommentMinGap, fmtconfig.Defaults())
	if l := lineContaining(t, got, "two"); l != "        bb         // two" {
		t.Errorf("got %q", l)
	}
}

func TestColumnForIgnoresOutliers(t *testing.T) {
	cfg := fmtconfig.Defaults() // ratio 2.5, min 40

	// Mean below outlier_min: the guard is inert and the widest body wins.
	if got := columnFor([]int{10, 12, 25}, annotationMinGap, cfg); got != 27 {
		t.Errorf("columnFor(small population) = %d, want 27", got)
	}

	// Mean above outlier_min with one extreme member: the extreme is
	// excluded. geomean(50,50,50,50,300) = 71.55, limit = 178.87, so 300 is
	// out and the four 50s set the column at 50+2.
	widths := []int{50, 50, 50, 50, 300}
	if got := columnFor(widths, annotationMinGap, cfg); got != 52 {
		t.Errorf("columnFor(with outlier) = %d, want 52 (outlier excluded)", got)
	}
}

func TestColumnForDisabledGuard(t *testing.T) {
	cfg := fmtconfig.Defaults()
	cfg.Align.OutlierRatio = 0
	widths := []int{50, 50, 50, 50, 300}
	if got := columnFor(widths, annotationMinGap, cfg); got != 302 {
		t.Errorf("columnFor(ratio=0) = %d, want 302 (guard disabled, plain max)", got)
	}
}

// TestColumnForUsesCallerMinGap proves the guard did not re-hardcode the
// annotation gap: the same widths and the same excluded outlier, but with
// minGap = 1 (the trailing-comment gap), must land the column one lower than
// the minGap = 2 case above.
func TestColumnForUsesCallerMinGap(t *testing.T) {
	cfg := fmtconfig.Defaults()
	widths := []int{50, 50, 50, 50, 300}
	if got := columnFor(widths, trailingCommentMinGap, cfg); got != 51 {
		t.Errorf("columnFor(minGap=1) = %d, want 51", got)
	}
}

// TestColumnFor_SmallDeviationIsNotAnOutlier pins the concrete case from the
// task: a 26-wide body in a population averaging ~10 must not be excluded by
// any ratio test. It survives here because the population's geometric mean
// (~10.45) never clears OutlierMin (40), so the guard never even computes a
// ratio against it: this is ordinary variation, not an extreme.
func TestColumnFor_SmallDeviationIsNotAnOutlier(t *testing.T) {
	cfg := fmtconfig.Defaults()
	widths := []int{5, 8, 10, 12, 26}
	if got := columnFor(widths, annotationMinGap, cfg); got != 28 {
		t.Errorf("columnFor(small deviation) = %d, want 28 (26 kept, guard inert)", got)
	}
}

// TestColumnFor_AllExcludedFallsBackToMaxWidth guards the degenerate case
// directly: if every width in a run were excluded, columnFor must fall back
// to the plain max rather than returning zero and collapsing every cell to
// minGap. With the default ratio (2.5, > 1) this can never actually happen:
// min <= geomean <= max always, so the smallest member is never excluded.
// A config with a sub-1 ratio (Validate only rejects negative) forces it:
// geomean(50,50,50) = 50, limit = 0.5*50 = 25, and every member of the run is
// above that.
func TestColumnFor_AllExcludedFallsBackToMaxWidth(t *testing.T) {
	cfg := fmtconfig.Defaults()
	cfg.Align.OutlierRatio = 0.5
	if got := columnFor([]int{50, 50, 50}, annotationMinGap, cfg); got != 52 {
		t.Errorf("columnFor(every member excluded) = %d, want 52 (fall back to plain max, never zero)", got)
	}
}

// TestAlignCells_OutlierGuardIsIdempotent exercises the guard through
// alignCells rather than columnFor directly, on lines carrying real
// annotations. It pins two things at once: which lines the guard puts in the
// column (the four 54-wide bodies, not the 304-wide one), and that running
// the aligner a second time makes the identical choice. splitAnnotation
// trims trailing space off the body before measuring it, so the widths
// columnFor sees on pass two are the same as on pass one regardless of what
// padding pass one wrote: a line excluded once must stay excluded.
func TestAlignCells_OutlierGuardIsIdempotent(t *testing.T) {
	body := func(n int) string { return strings.Repeat("x", n) }
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    " + body(50) + " [GET /v1/x]\n" +
		"    " + body(50) + " [GET /v1/x]\n" +
		"    " + body(50) + " [GET /v1/x]\n" +
		"    " + body(50) + " [GET /v1/x]\n" +
		"    " + body(300) + " [GET /v1/x]\n" +
		"}\n"

	once := alignCells(in, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
	twice := alignCells(once, nil, nil, splitAnnotation, fmtconfig.ScopeBlock, annotationMinGap, fmtconfig.Defaults())
	if once != twice {
		t.Errorf("not idempotent under outlier guard\nonce:\n%s\ntwice:\n%s", once, twice)
	}

	// The four 54-wide bodies (4 leading spaces + 50 x's) set the column;
	// the 304-wide outlier is excluded and gets only the minimum gap.
	wantGap := strings.Repeat(" ", 2)
	for _, l := range strings.Split(once, "\n") {
		if strings.HasPrefix(strings.TrimLeft(l, " "), body(50)+" ") {
			if !strings.Contains(l, body(50)+wantGap+"[GET") {
				t.Errorf("normal-width line not aligned to the guard's column: %q", l)
			}
		}
		if strings.HasPrefix(strings.TrimLeft(l, " "), body(300)) {
			if !strings.Contains(l, body(300)+" [GET") || strings.Contains(l, body(300)+"  [GET") {
				t.Errorf("outlier line should get only the minimum one-space gap: %q", l)
			}
		}
	}
}

// hintsFor builds the commentStart map the walker would produce, for tests that
// exercise the aligner directly rather than through the formatter.
func hintsFor(s string) map[int]int {
	m := map[int]int{}
	for i, l := range strings.Split(s, "\n") {
		if idx := strings.Index(l, "//"); idx > 0 && strings.TrimSpace(l[:idx]) != "" {
			m[i] = idx
		}
	}
	return m
}

func lineContaining(t *testing.T, s, sub string) string {
	t.Helper()
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	t.Fatalf("no line containing %q in\n%s", sub, s)
	return ""
}
