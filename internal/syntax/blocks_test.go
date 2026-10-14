package syntax_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const blocksSrc = `use_case "U" {
    when CRON fires the schedule
        sp asks nmf for the range
        alt "nMF has a cycle"
            sp returns the cycle
        else "nMF answers 404"
            sp answers not-found
        else "nMF is unhealthy"
            sp retries once
            opt "the retry also fails"
                sp answers a server error
        sp logs the run
}
`

const blocksTabsSrc = "use_case \"U\" {\n\twhen Ops does it\n\t\talt \"a\"\n\t\t\tx does a\n\t\telse \"b\"\n\t\t\tx does b\n\t\tx ends\n}"

// outline renders the step tree compactly.
func outline(steps []syntax.StepDecl, depth int) string {
	var b strings.Builder
	for _, st := range steps {
		ind := strings.Repeat("  ", depth)
		if a, ok := st.Action(); ok {
			b.WriteString(ind + "action " + a.SubjectName() + "\n")
			continue
		}
		blk, _ := st.Block()
		b.WriteString(ind + string(blk.Kind()) + "\n")
		for _, br := range blk.Branches() {
			b.WriteString(ind + "  branch " + br.Label() + "\n")
			b.WriteString(outline(br.Steps(), depth+2))
		}
	}
	return b.String()
}

// outlineAll renders every scenario of the first use case, one "when" line
// per scenario followed by its step outline.
func outlineAll(t *testing.T, src string) (string, []model.Diagnostic) {
	t.Helper()
	g, _, diags := syntax.Parse(src)
	var b strings.Builder
	for _, sc := range syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios() {
		b.WriteString("when\n")
		b.WriteString(outline(sc.Steps(), 1))
	}
	return b.String(), diags
}

func codes(diags []model.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func TestBlocks_TreeShape(t *testing.T) {
	g, _, diags := syntax.Parse(blocksSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	want := "action sp\nalt\n  branch nMF has a cycle\n    action sp\n  branch nMF answers 404\n    action sp\n  branch nMF is unhealthy\n    action sp\n    opt\n      branch the retry also fails\n        action sp\naction sp\n"
	if got := outline(sc.Steps(), 0); got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	if n := len(sc.Actions()); n != 6 {
		t.Errorf("Actions() = %d, want 6 (flattened)", n)
	}
}

func TestBlocks_ActionsFlattenedInSourceOrder(t *testing.T) {
	g, _, _ := syntax.Parse(blocksSrc)
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	var verbs []string
	for _, a := range sc.Actions() {
		verbs = append(verbs, a.VerbValue())
	}
	want := "asks returns answers retries answers logs"
	if got := strings.Join(verbs, " "); got != want {
		t.Errorf("verbs = %q, want %q", got, want)
	}
}

func TestBlocks_BranchViews(t *testing.T) {
	g, _, _ := syntax.Parse(blocksSrc)
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	blk, ok := sc.Steps()[1].Block()
	if !ok {
		t.Fatal("step 1 is not a block")
	}
	if blk.Node().Kind() != syntax.SyntaxKindBlock {
		t.Errorf("block node kind = %v", blk.Node().Kind())
	}
	brs := blk.Branches()
	if len(brs) != 3 {
		t.Fatalf("branches = %d, want 3", len(brs))
	}
	if brs[0].Keyword() != "alt" || brs[1].Keyword() != "else" {
		t.Errorf("keywords = %q, %q", brs[0].Keyword(), brs[1].Keyword())
	}
	ht := brs[1].HeaderToken()
	if ht == nil || ht.Kind() != syntax.SyntaxKindKwElse || ht.Text() != "else" {
		t.Errorf("header token = %v", ht)
	}
	if _, ok := sc.Steps()[0].Block(); ok {
		t.Error("step 0 must not be a block")
	}
}

func TestBlocks_KeywordsStayUsableAsNames(t *testing.T) {
	src := "use_case \"U\" {\n  when Ops does it\n    alt asks opt to run\n    else records something\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	if got := outline(sc.Steps(), 0); got != "action alt\naction else\n" {
		t.Errorf("tree:\n%s", got)
	}
}

func TestBlocks_TabsCountAsFourColumns(t *testing.T) {
	g, _, diags := syntax.Parse(blocksTabsSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	want := "alt\n  branch a\n    action x\n  branch b\n    action x\naction x\n"
	if got := outline(sc.Steps(), 0); got != want {
		t.Errorf("tree:\n%s", got)
	}
}

// A tab advances to the next multiple of 4: "  \t" (2 spaces + tab) lands on
// the same visual column as "\t" and as four spaces.
func TestBlocks_TabAfterSpacesAlignsToTabStop(t *testing.T) {
	src := "use_case \"U\" {\n  when Ops does it\n    alt \"a\"\n      x does a\n  \telse \"b\"\n      x does b\n}"
	got, diags := outlineAll(t, src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	want := "when\n  alt\n    branch a\n      action x\n    branch b\n      action x\n"
	if got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
}

func TestBlocks_Lossless(t *testing.T) {
	checkLossless(t, blocksSrc)
	checkLossless(t, blocksTabsSrc)
}

func TestBlocks_EndAtScenarioBoundary(t *testing.T) {
	cases := map[string]string{
		"when":     "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n        when P does\n      y does it\n}",
		"scenario": "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n        scenario \"s\"\n        when P does\n      y does it\n}",
	}
	want := "when\n  alt\n    branch a\n      action x\nwhen\n  action y\n"
	for name, src := range cases {
		got, diags := outlineAll(t, src)
		if len(diags) != 0 {
			t.Errorf("%s: unexpected diags: %v", name, diags)
		}
		if got != want {
			t.Errorf("%s: tree:\n%s\nwant:\n%s", name, got, want)
		}
	}

	// `}` indented deeper than the header still closes the use case.
	src := "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n        }\nuse_case \"V\" {\n  when P does\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("brace: unexpected diags: %v", diags)
	}
	if n := len(syntax.AsFile(syntax.Root(g)).UseCases()); n != 2 {
		t.Errorf("brace: use cases = %d, want 2", n)
	}
}

func TestBlocks_CommentsAndBlankLinesDoNotEndBody(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n\n// shallow comment\n      // deep comment\n\n      y does it\n    z ends\n}"
	got, diags := outlineAll(t, src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	want := "when\n  alt\n    branch a\n      action x\n      action y\n  action z\n"
	if got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	checkLossless(t, src)
}

func TestBlocks_Recovery(t *testing.T) {
	cases := []struct {
		name, src, code, tree string
		line, char, endChar   int // 0-based LSP range of the diagnostic
	}{
		{
			name: "empty-branch continues at parent level",
			src:  "use_case \"U\" {\n  when O does\n    alt \"a\"\n    x does it\n}",
			code: "craft/syntax/empty-branch",
			tree: "when\n  alt\n    branch a\n  action x\n",
			line: 2, char: 4, endChar: 7,
		},
		{
			name: "empty else branch",
			src:  "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n    else \"b\"\n    y does it\n}",
			code: "craft/syntax/empty-branch",
			tree: "when\n  alt\n    branch a\n      action x\n    branch b\n  action y\n",
			line: 4, char: 4, endChar: 8,
		},
		{
			name: "inconsistent-indent joins the innermost block left of it",
			src:  "use_case \"U\" {\n  when O does\n    alt \"a\"\n        x does it\n      y does more\n    z ends\n}",
			code: "craft/syntax/inconsistent-indent",
			tree: "when\n  alt\n    branch a\n      action x\n      action y\n  action z\n",
			line: 4, char: 6, endChar: 7,
		},
		{
			name: "inconsistent-indent in a nested block joins the outer one",
			src:  "use_case \"U\" {\n  when O does\n    alt \"a\"\n        x does it\n        opt \"b\"\n            y does it\n      w does more\n}",
			code: "craft/syntax/inconsistent-indent",
			tree: "when\n  alt\n    branch a\n      action x\n      opt\n        branch b\n          action y\n      action w\n",
			line: 6, char: 6, endChar: 7,
		},
		{
			name: "orphan-else after opt starts a new alt",
			src:  "use_case \"U\" {\n  when O does\n    opt \"a\"\n      x does it\n    else \"b\"\n      x does more\n}",
			code: "craft/syntax/orphan-else",
			tree: "when\n  opt\n    branch a\n      action x\n  alt\n    branch b\n      action x\n",
			line: 4, char: 4, endChar: 8,
		},
		{
			name: "orphan-else with no alt",
			src:  "use_case \"U\" {\n  when O does\n    x does it\n    else \"b\"\n      y does more\n    else \"c\"\n      z does more\n}",
			code: "craft/syntax/orphan-else",
			tree: "when\n  action x\n  alt\n    branch b\n      action y\n    branch c\n      action z\n",
			line: 3, char: 4, endChar: 8,
		},
		{
			name: "missing-label gives an empty label",
			src:  "use_case \"U\" {\n  when O does\n    alt\n      x does it\n}",
			code: "craft/syntax/missing-label",
			tree: "when\n  alt\n    branch \n      action x\n",
			line: 2, char: 4, endChar: 7,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := outlineAll(t, tc.src)
			if len(diags) != 1 || diags[0].Code != tc.code {
				t.Fatalf("diags = %v, want exactly one %s", codes(diags), tc.code)
			}
			d := diags[0]
			if d.Severity != model.SeverityError {
				t.Errorf("severity = %v", d.Severity)
			}
			if d.Range.Start.Line != tc.line || d.Range.Start.Character != tc.char ||
				d.Range.End.Line != tc.line || d.Range.End.Character != tc.endChar {
				t.Errorf("range = %+v, want line %d chars %d-%d", d.Range, tc.line, tc.char, tc.endChar)
			}
			if got != tc.tree {
				t.Errorf("tree:\n%s\nwant:\n%s", got, tc.tree)
			}
			checkLossless(t, tc.src)
		})
	}
}

func TestBlocks_DiagnosticMessages(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n  when O does\n    opt \"a\"\n    x does it\n}")
	if len(diags) != 1 || diags[0].Message != "`opt \"…\"` has no steps: indent its steps under it" {
		t.Errorf("empty-branch message: %v", diags)
	}
}

// An unterminated block at EOF reports only the use case's missing `}`.
func TestBlocks_UnterminatedAtEOF(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n")
	if got := strings.Join(codes(diags), ","); got != "craft/syntax/unclosed-block" {
		t.Errorf("diags = %s", got)
	}
}

// Tokens after the label on a header line are reported and kept out of both
// the label and the body.
func TestBlocks_TrailingTokensAfterLabel(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt \"a\" x does it\n      y does it\n}"
	got, diags := outlineAll(t, src)
	if c := strings.Join(codes(diags), ","); c != "craft/syntax/unexpected-token" {
		t.Fatalf("diags = %s", c)
	}
	if want := "when\n  alt\n    branch a\n      action y\n"; got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	checkLossless(t, src)
}

// An action's verb must be on the action's own line: a subject-only line
// does not borrow the next line's first token as its verb, so a block body
// cannot swallow the line after it.
func TestBlocks_VerbMustBeOnActionLine(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt \"c\"\n      x\n    y does z\n}"
	got, diags := outlineAll(t, src)
	if c := strings.Join(codes(diags), ","); c != "craft/syntax/unexpected-token" {
		t.Fatalf("diags = %s", c)
	}
	r := diags[0].Range
	if r.Start.Line != 3 || r.Start.Character != 6 || r.End.Character != 7 {
		t.Errorf("range = %+v, want line 3 chars 6-7 (the subject)", r)
	}
	if want := "when\n  alt\n    branch c\n      action x\n  action y\n"; got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	checkLossless(t, src)
}

// A block comment before the keyword is trivia: the line is still a header.
func TestBlocks_BlockCommentBeforeHeader(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n    /* c */ else \"b\"\n      y does it\n    /* d */ opt \"e\"\n      z does it\n}"
	got, diags := outlineAll(t, src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	want := "when\n  alt\n    branch a\n      action x\n    branch b\n      action y\n  opt\n    branch e\n      action z\n"
	if got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	checkLossless(t, src)
}

// An else that is both mis-indented and orphaned reports only orphan-else.
func TestBlocks_MisindentedOrphanElseReportsOnlyOrphan(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt \"a\"\n        x does it\n      else \"b\"\n        y does it\n}"
	got, diags := outlineAll(t, src)
	if c := strings.Join(codes(diags), ","); c != "craft/syntax/orphan-else" {
		t.Fatalf("diags = %s, want only orphan-else", c)
	}
	want := "when\n  alt\n    branch a\n      action x\n      alt\n        branch b\n          action y\n"
	if got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
}

// longBodyLineSrc is a block body line of n error steps (`"s"` tokens). Each
// token is its own step, so any per-step work proportional to the line length
// makes the parse quadratic.
func longBodyLineSrc(n int) string {
	return "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n      " +
		strings.Repeat("\"s\" ", n) + "\n}"
}

// bestParseTime returns the fastest of three parses of src.
func bestParseTime(src string) time.Duration {
	best := time.Duration(1<<63 - 1)
	for range 3 {
		start := time.Now()
		syntax.Parse(src)
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best
}

// Linear parsing takes about 4x as long for 4x the tokens; quadratic about
// 16x. A ratio, not an absolute bound, keeps this stable on slow machines.
func TestBlocks_LongLineIsNotQuadratic(t *testing.T) {
	const n = 5000
	small := bestParseTime(longBodyLineSrc(n))
	large := bestParseTime(longBodyLineSrc(4 * n))
	if large >= 10*small {
		t.Errorf("parse time grew %.1fx for 4x the tokens (%v -> %v), want < 10x (linear is ~4x)",
			float64(large)/float64(small), small, large)
	}
}

func BenchmarkBlocks_LongLine(b *testing.B) {
	src := longBodyLineSrc(20000)
	for b.Loop() {
		syntax.Parse(src)
	}
}

// The no-verb diagnostic spans the whole subject slot, qualified name included.
func TestBlocks_NoVerbSpansQualifiedSubject(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    re/billing\n    y does z\n}"
	got, diags := outlineAll(t, src)
	if c := strings.Join(codes(diags), ","); c != "craft/syntax/unexpected-token" {
		t.Fatalf("diags = %s", c)
	}
	r := diags[0].Range
	if r.Start.Line != 2 || r.Start.Character != 4 || r.End.Line != 2 || r.End.Character != 14 {
		t.Errorf("range = %+v, want line 2 chars 4-14", r)
	}
	if !strings.Contains(diags[0].Message, `"re/billing"`) {
		t.Errorf("message = %q, want the full subject name", diags[0].Message)
	}
	if want := "when\n  action re/billing\n  action y\n"; got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
}

// Multi-line block comments before a header keyword. These pin current
// behaviour: comments are trivia, so a keyword preceded on its line only by
// the tail of a block comment that began on an earlier line is still a
// header, and the offside rule measures that line by its indentation, which
// is where the comment's tail starts.
func TestBlocks_MultiLineCommentBeforeHeader(t *testing.T) {
	cases := map[string]struct{ src, tree string }{
		// The comment starts on its own line; ` */` is indented like the alt.
		"comment-only line before": {
			src:  "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n    /* a\n    b */ else \"b\"\n      y does it\n}",
			tree: "when\n  alt\n    branch a\n      action x\n    branch b\n      action y\n",
		},
		// The comment starts after an action on the previous line; its tail
		// at column 2 is the header line's indentation.
		"comment opened after an action": {
			src:  "use_case \"U\" {\n  when O does\n    x does it /* c\n */ alt \"b\"\n      y does it\n}",
			tree: "when\n  action x\n  alt\n    branch b\n      action y\n",
		},
	}
	for name, tc := range cases {
		got, diags := outlineAll(t, tc.src)
		if len(diags) != 0 {
			t.Errorf("%s: unexpected diags: %v", name, diags)
		}
		if got != tc.tree {
			t.Errorf("%s: tree:\n%s\nwant:\n%s", name, got, tc.tree)
		}
		checkLossless(t, tc.src)
	}
}

// An else at the outer alt's column closes the nested alt (and its else) and
// adds a branch to the outer one.
func TestBlocks_ElseAtOuterColumnAfterNestedAlt(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n      alt \"b\"\n        y does it\n      else \"c\"\n        z does it\n    else \"d\"\n      w does it\n    v ends\n}"
	got, diags := outlineAll(t, src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	want := "when\n  alt\n    branch a\n      action x\n      alt\n        branch b\n          action y\n        branch c\n          action z\n    branch d\n      action w\n  action v\n"
	if got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
}

// A label-less, body-less header as the last line before EOF: missing label,
// empty branch, and the use case's own missing `}`.
func TestBlocks_BareHeaderAtEOF(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    x does it\n    alt\n"
	got, diags := outlineAll(t, src)
	if c := strings.Join(codes(diags), ","); c != "craft/syntax/missing-label,craft/syntax/empty-branch,craft/syntax/unclosed-block" {
		t.Errorf("diags = %s", c)
	}
	if want := "when\n  action x\n  alt\n    branch \n"; got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	checkLossless(t, src)
}

// A label on the line after the keyword is not a label. This pins the current
// three diagnostics: the header has no label, its branch is empty (the string
// line is not deeper than the header), and the string is not a step.
func TestBlocks_LabelOnNextLine(t *testing.T) {
	src := "use_case \"U\" {\n  when O does\n    alt\n    \"label\"\n      x does it\n}"
	got, diags := outlineAll(t, src)
	if c := strings.Join(codes(diags), ","); c != "craft/syntax/missing-label,craft/syntax/empty-branch,craft/syntax/unexpected-token" {
		t.Errorf("diags = %s", c)
	}
	if want := "when\n  alt\n    branch \n  action \n  action x\n"; got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	checkLossless(t, src)
}
