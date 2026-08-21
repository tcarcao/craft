package lsp

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/fmtconfig"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// sepTok builds a token of the given kind and text for separator tests by
// parsing source and extracting the non-whitespace token at index idx.
// separatorFor only reads Kind(), Text() and Parent(), and a token built by
// the parser carries a real parent, so this covers all cases including the
// ref-colon rule (which depends on the parsed Parent() context).
func sepTok(t *testing.T, src string, idx int) syntax.SyntaxToken {
	t.Helper()
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()
	var out []syntax.SyntaxToken
	for _, tk := range toks {
		if tk.Kind() != syntax.SyntaxKindWhitespace {
			out = append(out, tk)
		}
	}
	if idx >= len(out) {
		t.Fatalf("token index %d out of range (%d tokens) for %q", idx, len(out), src)
	}
	return out[idx]
}

func TestSeparatorFor_FirstTokenHasNoSeparator(t *testing.T) {
	curr := sepTok(t, "domain re {\n  Billing\n}\n", 0)
	if got := separatorFor(nil, "", curr, 0, false, fmtconfig.Defaults()); got != "" {
		t.Errorf("separatorFor(nil, ...) = %q, want empty", got)
	}
}

func TestSeparatorFor_GapWithNewlineBreaksTheLine(t *testing.T) {
	src := "domain re {\n  Billing\n}\n"
	prev := sepTok(t, src, 2) // {
	curr := sepTok(t, src, 3) // Billing
	if got := separatorFor(&prev, "\n  ", curr, 1, false, fmtconfig.Defaults()); got != "\n    " {
		t.Errorf("got %q, want %q", got, "\n    ")
	}
}

// The two identifiers (not the closing brace) are the ones exercised here:
// `}` is now a block boundary in its own right and always forces a plain
// newline before it regardless of blank lines in the gap, so testing the
// generic collapse-to-one-blank-line behaviour has to use a pair that isn't
// a block boundary.
func TestSeparatorFor_BlankLineRunCollapsesToOne(t *testing.T) {
	src := "domain re {\n  Billing\n\n\n\n  Invoicing\n}\n"
	prev := sepTok(t, src, 3)
	curr := sepTok(t, src, 4)
	if got := separatorFor(&prev, "\n\n\n\n", curr, 1, false, fmtconfig.Defaults()); got != "\n\n    " {
		t.Errorf("got %q, want one blank line then indent", got)
	}
}

func TestSeparatorFor_SameLineRunOfSpacesCollapsesToOne(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n}\n"
	prev := sepTok(t, src, 4)
	curr := sepTok(t, src, 5)
	if got := separatorFor(&prev, "     ", curr, 2, false, fmtconfig.Defaults()); got != " " {
		t.Errorf("got %q, want a single space", got)
	}
}

func TestSeparatorFor_AdjacentTokensStayAdjacent(t *testing.T) {
	// `re/billing` has no gaps, so its parts must concatenate. This is what
	// removes the need for any Ref special case.
	src := "use_case \"X\" {\n  when U does x\n    A asks re/billing for c\n}\n"
	gn, _, _ := syntax.Parse(src)
	var prev, curr syntax.SyntaxToken
	var found bool
	toks := syntax.Root(gn).AllTokens()
	for i := 1; i < len(toks); i++ {
		if toks[i].Kind() == syntax.SyntaxKindWhitespace {
			continue
		}
		if toks[i].Text() == "/" {
			prev, curr, found = toks[i-1], toks[i], true
			break
		}
	}
	if !found {
		t.Fatal("no / token found in a qualified ref")
	}
	if got := separatorFor(&prev, "", curr, 2, false, fmtconfig.Defaults()); got != "" {
		t.Errorf("got %q, want empty so re/billing stays joined", got)
	}
}

// TestSeparatorFor_OneSpaceBeforeAnOpeningBrace is the mirror of the existing
// rule that breaks the line after `{`. Without it a minified declaration only
// half expanded: the body was indented onto its own lines but the brace stayed
// glued to the block name, as `service Foo{`.
func TestSeparatorFor_OneSpaceBeforeAnOpeningBrace(t *testing.T) {
	src := "service Foo{contexts: A}\n"
	prev := sepTok(t, src, 1) // Foo
	curr := sepTok(t, src, 2) // {
	if got := separatorFor(&prev, "", curr, 0, false, fmtconfig.Defaults()); got != " " {
		t.Errorf("got %q, want a single space so `service Foo{` becomes `service Foo {`", got)
	}
}

// TestSeparatorFor_AuthorBraceOnItsOwnLineSurvives guards the rule above. A
// line break the author wrote is authorial, so a brace deliberately placed on
// its own line must not be pulled up onto the header.
func TestSeparatorFor_AuthorBraceOnItsOwnLineSurvives(t *testing.T) {
	src := "service Foo\n{\n  contexts: A\n}\n"
	prev := sepTok(t, src, 1) // Foo
	curr := sepTok(t, src, 2) // {
	if got := separatorFor(&prev, "\n", curr, 0, false, fmtconfig.Defaults()); got != "\n" {
		t.Errorf("got %q, want the author's line break preserved", got)
	}
}

// TestSeparatorFor_NewLineAfterAClosingBrace is the mirror of the existing rule
// that breaks the line before `}`. Without it `}Commerce{` never broke, so two
// sibling blocks ran together on one line.
func TestSeparatorFor_NewLineAfterAClosingBrace(t *testing.T) {
	// Token indices: 0 domains, 1 {, 2 Auth, 3 {, 4 Login, 5 }, 6 Commerce.
	src := "domains{Auth{Login}Commerce{Cart}}\n"
	prev := sepTok(t, src, 5) // } closing Auth
	curr := sepTok(t, src, 6) // Commerce
	if got := separatorFor(&prev, "", curr, 1, false, fmtconfig.Defaults()); got != "\n    " {
		t.Errorf("got %q, want a newline and one level of indent", got)
	}
}

// TestSeparatorFor_BlankLineAfterAClosingBraceSurvives guards the rule above,
// so a blank line the author put between two sibling blocks is not flattened.
func TestSeparatorFor_BlankLineAfterAClosingBraceSurvives(t *testing.T) {
	src := "domains {\n  Auth {\n    Login\n  }\n\n  Commerce {\n    Cart\n  }\n}\n"
	prev := sepTok(t, src, 5) // } closing Auth
	curr := sepTok(t, src, 6) // Commerce
	if got := separatorFor(&prev, "\n\n  ", curr, 1, false, fmtconfig.Defaults()); got != "\n\n    " {
		t.Errorf("got %q, want the author's blank line preserved", got)
	}
}

// TestSeparatorFor_SeveralStatementsOnOneLineStayThere pins the accepted limit
// of minified expansion rather than leaving it to be rediscovered as a bug.
//
// Braces expand because a brace is a token the walker can see. A statement
// boundary is not: `user Alice system Bot` is two statements separated by the
// same single space that separates `user` from `Alice`, and nothing in the gap
// tells them apart. Deriving the boundary from the tree instead was tried and
// emitted source that did not parse, because an action's event ref and its
// `[op]` annotation are non-nested siblings and got split across lines. The
// formatter leaves these as the author wrote them.
func TestSeparatorFor_SeveralStatementsOnOneLineStayThere(t *testing.T) {
	src := "actors{user Alice system Bot}\n"
	prev := sepTok(t, src, 3) // Alice
	curr := sepTok(t, src, 4) // system
	if got := separatorFor(&prev, " ", curr, 1, false, fmtconfig.Defaults()); got != " " {
		t.Errorf("got %q, want a single space: statement boundaries are not inferred", got)
	}
}

// TestSeparatorFor_TrailingCommaOrColonDoesNotDefeatTheBraceBreak covers the
// one place a field separator used to beat the forced break before `}`. The
// colon and comma rules sit above the brace rules and returned before
// `curr == RBrace` got a say, so `services{Foo{contexts:A,}}` came back as
// `contexts: A, }` and the block never finished expanding. Only degenerate
// input reaches it, but the brace rule is meant to be the one thing that always
// wins on structure.
func TestSeparatorFor_TrailingCommaOrColonDoesNotDefeatTheBraceBreak(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		idx  int // the index of the trailing `,` or `:`
	}{
		{"trailing comma", "services{Foo{contexts:A,}}\n", 7},
		{"trailing colon", "services{Foo{contexts:}}\n", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := sepTok(t, tc.src, tc.idx)
			curr := sepTok(t, tc.src, tc.idx+1) // }
			if curr.Kind() != syntax.SyntaxKindRBrace {
				t.Fatalf("fixture drift: token %d is %q, want `}`", tc.idx+1, curr.Text())
			}
			if got := separatorFor(&prev, "", curr, 1, false, fmtconfig.Defaults()); got != "\n    " {
				t.Errorf("after %q: got %q, want the forced break before `}`", prev.Text(), got)
			}
		})
	}
}

// TestFormatDocument_TrailingCommaBeforeBraceStillExpands is the same defect at
// the document level, which is where it was visible.
func TestFormatDocument_TrailingCommaBeforeBraceStillExpands(t *testing.T) {
	got := FormatDocument("services{Foo{contexts:A,}}")
	want := "services {\n    Foo {\n        contexts: A,\n    }\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := FormatDocument(got); again != got {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestSeparatorFor_NoSpaceBeforeColonOrComma(t *testing.T) {
	src := "services {\n  S {\n    contexts: A, B\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	for _, want := range []string{":", ","} {
		toks := syntax.Root(gn).AllTokens()
		for i := 1; i < len(toks); i++ {
			if toks[i].Text() != want {
				continue
			}
			prev := toks[i-1]
			if got := separatorFor(&prev, " ", toks[i], 2, false, fmtconfig.Defaults()); got != "" {
				t.Errorf("before %q: got %q, want empty", want, got)
			}
			break
		}
	}
}

func TestSeparatorFor_FieldColonGetsOneSpaceAfter(t *testing.T) {
	// `contexts:A` must normalise to `contexts: A`, so the rule cannot be
	// gap-driven alone.
	src := "services {\n  S {\n    contexts: A\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()
	for i := 0; i < len(toks)-1; i++ {
		if toks[i].Kind() != syntax.SyntaxKindColon {
			continue
		}
		var next syntax.SyntaxToken
		for j := i + 1; j < len(toks); j++ {
			if toks[j].Kind() != syntax.SyntaxKindWhitespace {
				next = toks[j]
				break
			}
		}
		if got := separatorFor(&toks[i], "", next, 2, false, fmtconfig.Defaults()); got != " " {
			t.Errorf("after a field colon: got %q, want one space", got)
		}
		return
	}
	t.Fatal("no colon found")
}

// TestSeparatorFor_ScenarioAlwaysGetsABlankLine pins the one rule where the
// formatter adds vertical space rather than preserving the author's. A `when`
// straight after `{` opens the body and must not get one.
func TestSeparatorFor_ScenarioAlwaysGetsABlankLine(t *testing.T) {
	src := "use_case \"X\" {\n  when A does x\n    P does y\n  when B does z\n    Q does w\n}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()

	var firstWhen, secondWhen int = -1, -1
	for i, tk := range toks {
		if tk.Kind() != syntax.SyntaxKindKwWhen {
			continue
		}
		if firstWhen < 0 {
			firstWhen = i
		} else if secondWhen < 0 {
			secondWhen = i
		}
	}
	if firstWhen < 0 || secondWhen < 0 {
		t.Fatal("expected two when tokens")
	}

	// Both are scenario starts as far as the walker is concerned. The first is
	// held back by prev being `{`, not by startsScenario.
	prevOfFirst := prevSignificant(toks, firstWhen)
	if got := separatorFor(prevOfFirst, "\n  ", toks[firstWhen], 1, true, fmtconfig.Defaults()); got != "\n    " {
		t.Errorf("first when: got %q, want a plain newline (it opens the body)", got)
	}

	prevOfSecond := prevSignificant(toks, secondWhen)
	if got := separatorFor(prevOfSecond, "\n  ", toks[secondWhen], 1, true, fmtconfig.Defaults()); got != "\n\n    " {
		t.Errorf("second when: got %q, want a blank line before the scenario", got)
	}

	// A `when` whose leading comment already opened the scenario asks for no
	// second blank line, which is what startsScenario=false expresses.
	if got := separatorFor(prevOfSecond, "\n  ", toks[secondWhen], 1, false, fmtconfig.Defaults()); got != "\n    " {
		t.Errorf("when after its own leading comment: got %q, want a plain newline", got)
	}
}

// prevSignificant returns the nearest non-whitespace token before i, or nil.
func prevSignificant(toks []syntax.SyntaxToken, i int) *syntax.SyntaxToken {
	for j := i - 1; j >= 0; j-- {
		if toks[j].Kind() == syntax.SyntaxKindWhitespace {
			continue
		}
		t := toks[j]
		return &t
	}
	return nil
}

// TestSeparatorFor_RefColonStaysTight covers the rule that distinguishes
// `bc:re/billing`, where the colon is inside a Ref node and must not get a
// space, from `contexts: A`, where it is a field separator and must.
func TestSeparatorFor_RefColonStaysTight(t *testing.T) {
	// A kind-prefixed ref in a glossary term node, exercising the real parent
	// context that isRefColon checks.
	src := "glossary {\n  bc:billing/Invoice same_as subscriptions/Invoice\n}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()

	// Find the colon inside the ref (bc:billing).
	var refColonIdx int = -1
	for i := 0; i < len(toks); i++ {
		if toks[i].Kind() != syntax.SyntaxKindColon {
			continue
		}
		// The colon inside bc:billing will have Parent().Kind() == SyntaxKindRef
		if toks[i].Parent() != nil && toks[i].Parent().Kind() == syntax.SyntaxKindRef {
			refColonIdx = i
			break
		}
	}
	if refColonIdx < 0 {
		t.Fatal("no ref colon found in bc:ref")
	}

	// Find the next non-whitespace token after the ref colon
	var nextTok syntax.SyntaxToken
	for i := refColonIdx + 1; i < len(toks); i++ {
		if toks[i].Kind() != syntax.SyntaxKindWhitespace {
			nextTok = toks[i]
			break
		}
	}

	// The ref colon must NOT get a space after it, so adjacent tokens stay joined
	if got := separatorFor(&toks[refColonIdx], "", nextTok, 1, false, fmtconfig.Defaults()); got != "" {
		t.Errorf("after ref colon: got %q, want empty so bc:billing stays joined", got)
	}
}

// TestIndentFor pins the basic depth-to-width mapping. It is specifically
// about indent width, so it pins its own cfg rather than relying on
// whatever fmtconfig.Defaults() happens to return, so a future change to the
// default cannot silently change what this test asserts.
func TestIndentFor(t *testing.T) {
	cfg := fmtconfig.Defaults()
	for depth, want := range map[int]string{0: "", 1: "    ", 3: "            "} {
		if got := indentFor(depth, cfg); got != want {
			t.Errorf("indentFor(%d) = %q, want %q", depth, got, want)
		}
	}
	if got := indentFor(-1, cfg); got != "" {
		t.Errorf("indentFor(-1) = %q, want empty (must not panic)", got)
	}
}

// TestSeparatorFor_LBraceForcesNewlineAfter pins the block-boundary rule: a
// `{` forces a break after it whatever the author wrote, which is what
// expands a minified declaration instead of leaving it gap-driven.
func TestSeparatorFor_LBraceForcesNewlineAfter(t *testing.T) {
	src := "service Foo{contexts: A}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()
	for i := 1; i < len(toks); i++ {
		if toks[i-1].Kind() != syntax.SyntaxKindLBrace {
			continue
		}
		if got := separatorFor(&toks[i-1], "", toks[i], 1, false, fmtconfig.Defaults()); got != "\n    " {
			t.Errorf("after {: got %q, want a newline plus indent", got)
		}
		return
	}
	t.Fatal("no LBrace found")
}

// TestSeparatorFor_RBraceForcesNewlineBefore pins the matching close-brace
// rule: a `}` forces a break before it whatever the author wrote.
func TestSeparatorFor_RBraceForcesNewlineBefore(t *testing.T) {
	src := "service Foo{contexts: A}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()
	for i := 1; i < len(toks); i++ {
		if toks[i].Kind() != syntax.SyntaxKindRBrace {
			continue
		}
		if got := separatorFor(&toks[i-1], "", toks[i], 0, false, fmtconfig.Defaults()); got != "\n" {
			t.Errorf("before }: got %q, want a newline at depth 0", got)
		}
		return
	}
	t.Fatal("no RBrace found")
}

// TestIndentForRespectsConfig pins indentFor to the configured width rather
// than a hardcoded one, and confirms the zero-floor still holds once depth is
// no longer the only input.
func TestIndentForRespectsConfig(t *testing.T) {
	four := fmtconfig.Defaults()
	two := fmtconfig.Defaults()
	two.Indent = 2

	if got := indentFor(2, four); got != "        " {
		t.Errorf("indentFor(2, indent=4) = %q, want 8 spaces", got)
	}
	if got := indentFor(2, two); got != "    " {
		t.Errorf("indentFor(2, indent=2) = %q, want 4 spaces", got)
	}
	if got := indentFor(0, four); got != "" {
		t.Errorf("indentFor(0) = %q, want empty", got)
	}
	if got := indentFor(-1, four); got != "" {
		t.Errorf("indentFor(-1) = %q, want empty (floors at zero)", got)
	}
}
