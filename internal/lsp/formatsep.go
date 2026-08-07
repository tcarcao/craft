package lsp

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// indentFor returns the indentation for a brace depth. It floors at zero: an
// unbalanced `}` only produces a warning-severity diagnostic, so it can reach
// the formatter with depth already at zero, and strings.Repeat panics on a
// negative count.
func indentFor(depth int) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat("  ", depth)
}

// separatorFor returns the whitespace to emit before curr.
//
// This is the entire formatting policy. Every other part of the formatter
// writes token text verbatim, so this function decides everything the
// formatter is allowed to decide.
//
// gap is the raw text of the whitespace token between prev and curr, or "" when
// they were adjacent in the source. Reading the author's gap rather than
// re-deriving line structure from the tree is what preserves author line breaks,
// and it is also what keeps a qualified ref joined: `re/billing` has no gaps, so
// every separator inside it is empty and the parts concatenate.
func separatorFor(prev *syntax.SyntaxToken, gap string, curr syntax.SyntaxToken, depth int) string {
	if prev == nil {
		return ""
	}

	// Never a space before a colon or comma, whatever the author wrote.
	if curr.Kind() == syntax.SyntaxKindColon || curr.Kind() == syntax.SyntaxKindComma {
		return ""
	}

	// One space after a field colon, even if the author wrote none, so
	// `contexts:A` normalises to `contexts: A`. A colon inside a ref
	// (`bc:re/billing`) is not a field colon and must stay tight, which the
	// parent check distinguishes. But when the author wrapped the value onto
	// its own line, that line break is intentional and must survive: falling
	// through to the newline handling below is what keeps
	// `contexts:\n  A` wrapped instead of collapsing it onto one line, and
	// matches the same rule for a comma immediately below.
	if prev.Kind() == syntax.SyntaxKindColon && !isRefColon(*prev) && !strings.Contains(gap, "\n") {
		return " "
	}
	// A comma normally gets one space after it, even if the author wrote
	// none, so `A,B` normalises to `A, B`. But when the author wrapped the
	// list across lines, that line break is intentional and must survive:
	// falling through to the newline handling below is what keeps
	// `contexts: A,\n  B` wrapped instead of collapsing it onto one line.
	if prev.Kind() == syntax.SyntaxKindComma && !strings.Contains(gap, "\n") {
		return " "
	}

	// A block boundary is structure, not authorial line breaking. Forcing the
	// break here is what expands a minified `service Foo{contexts: A}` into
	// three lines.
	//
	// All four rules sit below the colon and comma rules above, so a field
	// separator still wins: those decide the shape of `contexts: A, B` before a
	// brace ever gets a say.
	//
	// The two `curr` rules are the mirrors of the two `prev` rules. Without
	// them a minified declaration only half expanded: `service Foo{` kept the
	// brace glued to the name, and `}Commerce{` never broke at all.
	//
	// Both mirrors are guarded on the gap, for different reasons. Reading a
	// `\n` there as authorial is the walker's whole premise, so `service Foo\n{`
	// must keep the author's brace-on-its-own-line style rather than be pulled
	// up, and `}\n\n  Commerce {` must keep its blank line rather than be
	// flattened to a single newline. In both cases the newline handling below
	// already does the right thing, so the guard is simply "only act when the
	// author wrote no line break at all", which is exactly the minified case.
	//
	// These two rules are the whole of what minified expansion does. Several
	// statements crammed onto one line stay as the author wrote them: telling
	// `user Alice system Bot` apart from one statement needs a
	// statement-boundary notion the token stream does not carry, and deriving
	// one from the tree produced output that did not parse (it split an
	// action's event ref from its `[op]` annotation). That is a documented
	// limit, not a defect.
	if curr.Kind() == syntax.SyntaxKindLBrace && !strings.Contains(gap, "\n") {
		return " "
	}
	if prev.Kind() == syntax.SyntaxKindLBrace {
		return "\n" + indentFor(depth)
	}
	if curr.Kind() == syntax.SyntaxKindRBrace {
		return "\n" + indentFor(depth)
	}

	// A scenario always gets a blank line before it, even if the author wrote
	// none. This is the one place the formatter adds vertical space rather than
	// preserving it, and it matches what the previous formatter did. `when` at
	// the very start of a use_case body is not a new scenario, and prev being
	// `{` is how that case is recognised.
	//
	// This sits ABOVE the `prev == RBrace` rule below, and the order is load
	// bearing. A `tags { }` block followed immediately by a `when` puts a `}`
	// and a `when` next to each other. With the brace rule first, a minified
	// `}when` got a plain newline on the first pass and the blank line only on
	// the second, once the newline was in the gap, so formatting was not
	// idempotent for that shape.
	if curr.Kind() == syntax.SyntaxKindKwWhen && depth == 1 && prev.Kind() != syntax.SyntaxKindLBrace {
		return "\n\n" + indentFor(depth)
	}

	if prev.Kind() == syntax.SyntaxKindRBrace && !strings.Contains(gap, "\n") {
		return "\n" + indentFor(depth)
	}

	switch strings.Count(gap, "\n") {
	case 0:
		if gap == "" {
			return ""
		}
		// Collapse any run of spaces or tabs to one.
		return " "
	case 1:
		return "\n" + indentFor(depth)
	default:
		// Two or more newlines is a blank line. Collapse any longer run to one.
		return "\n\n" + indentFor(depth)
	}
}

// isRefColon reports whether tok is the `:` inside a node slug such as
// `bc:re/billing`, as opposed to a field separator such as `contexts:`.
func isRefColon(tok syntax.SyntaxToken) bool {
	parent := tok.Parent()
	return parent != nil && parent.Kind() == syntax.SyntaxKindRef
}
