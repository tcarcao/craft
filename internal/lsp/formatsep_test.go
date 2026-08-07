package lsp

import (
	"testing"

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
	if got := separatorFor(nil, "", curr, 0); got != "" {
		t.Errorf("separatorFor(nil, ...) = %q, want empty", got)
	}
}

func TestSeparatorFor_GapWithNewlineBreaksTheLine(t *testing.T) {
	src := "domain re {\n  Billing\n}\n"
	prev := sepTok(t, src, 2) // {
	curr := sepTok(t, src, 3) // Billing
	if got := separatorFor(&prev, "\n  ", curr, 1); got != "\n  " {
		t.Errorf("got %q, want %q", got, "\n  ")
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
	if got := separatorFor(&prev, "\n\n\n\n", curr, 1); got != "\n\n  " {
		t.Errorf("got %q, want one blank line then indent", got)
	}
}

func TestSeparatorFor_SameLineRunOfSpacesCollapsesToOne(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n}\n"
	prev := sepTok(t, src, 4)
	curr := sepTok(t, src, 5)
	if got := separatorFor(&prev, "     ", curr, 2); got != " " {
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
	if got := separatorFor(&prev, "", curr, 2); got != "" {
		t.Errorf("got %q, want empty so re/billing stays joined", got)
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
			if got := separatorFor(&prev, " ", toks[i], 2); got != "" {
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
		if got := separatorFor(&toks[i], "", next, 2); got != " " {
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

	prevOfFirst := prevSignificant(toks, firstWhen)
	if got := separatorFor(prevOfFirst, "\n  ", toks[firstWhen], 1); got != "\n  " {
		t.Errorf("first when: got %q, want a plain newline (it opens the body)", got)
	}

	prevOfSecond := prevSignificant(toks, secondWhen)
	if got := separatorFor(prevOfSecond, "\n  ", toks[secondWhen], 1); got != "\n\n  " {
		t.Errorf("second when: got %q, want a blank line before the scenario", got)
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
	if got := separatorFor(&toks[refColonIdx], "", nextTok, 1); got != "" {
		t.Errorf("after ref colon: got %q, want empty so bc:billing stays joined", got)
	}
}

func TestIndentFor(t *testing.T) {
	for depth, want := range map[int]string{0: "", 1: "  ", 3: "      "} {
		if got := indentFor(depth); got != want {
			t.Errorf("indentFor(%d) = %q, want %q", depth, got, want)
		}
	}
	if got := indentFor(-1); got != "" {
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
		if got := separatorFor(&toks[i-1], "", toks[i], 1); got != "\n  " {
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
		if got := separatorFor(&toks[i-1], "", toks[i], 0); got != "\n" {
			t.Errorf("before }: got %q, want a newline at depth 0", got)
		}
		return
	}
	t.Fatal("no RBrace found")
}
