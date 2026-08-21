package lsp

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/fmtconfig"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// indentFor returns the indentation for a brace depth. It floors at zero: an
// unbalanced `}` only produces a warning-severity diagnostic, so it can reach
// the formatter with depth already at zero, and strings.Repeat panics on a
// negative count.
//
// Width comes from the configuration rather than being fixed, because a
// workspace's existing files are the thing being formatted and half the known
// corpus is 4-space. Config.Validate bounds the width, so the multiplication
// here cannot be driven to exhaustion by a file on disk.
func indentFor(depth int, cfg fmtconfig.Config) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat(" ", cfg.Indent*depth)
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
// startsScenario is the walker's answer to "does a new scenario begin at this
// token". It cannot be derived here: the token that opens a scenario may be a
// comment rather than the `when`, and telling those apart needs lookahead to
// the comment's owner, which separatorFor never sees.
// continuing is the walker's answer to "is curr a continuation of a property
// value the author wrapped onto another line, rather than the start of a new
// statement". It cannot be derived here either, for the same reason: a
// wrapped `contexts: A,\n B` and two adjacent statements look identical to a
// function that only ever sees one token pair. It is passed in rather than
// measured, which is what keeps the continuation column a pure function of
// depth and cfg. See the block comment at the newline switch below.
func separatorFor(prev *syntax.SyntaxToken, gap string, curr syntax.SyntaxToken, depth int, startsScenario bool, cfg fmtconfig.Config, continuing bool) string {
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
	//
	// A `}` is excluded, and so is it in the comma rule below. Returning here
	// for a `}` meant a field separator won against the forced break before a
	// closing brace, and `services{Foo{contexts:A,}}` came back as
	// `contexts: A, }` on one line: the block never finished expanding. Only
	// degenerate input reaches it, since a trailing comma or colon with nothing
	// after it is a value the author has not written yet, but the brace rule is
	// meant to be the one thing that always wins on structure.
	if prev.Kind() == syntax.SyntaxKindColon && !isRefColon(*prev) &&
		!strings.Contains(gap, "\n") && curr.Kind() != syntax.SyntaxKindRBrace {
		return " "
	}
	// A comma normally gets one space after it, even if the author wrote
	// none, so `A,B` normalises to `A, B`. But when the author wrapped the
	// list across lines, that line break is intentional and must survive:
	// falling through to the newline handling below is what keeps
	// `contexts: A,\n  B` wrapped instead of collapsing it onto one line.
	if prev.Kind() == syntax.SyntaxKindComma && !strings.Contains(gap, "\n") &&
		curr.Kind() != syntax.SyntaxKindRBrace {
		return " "
	}

	// A block boundary is structure, not authorial line breaking. Forcing the
	// break here is what expands a minified `service Foo{contexts: A}` into
	// three lines.
	//
	// There are four brace rules. Three of them are these; the fourth,
	// `prev == RBrace`, does NOT sit with them. It lives below the scenario
	// rule, at the bottom of this function, and the reason it has to is given
	// at its own site. Precedence review has to read both places.
	//
	// These three sit below the colon and comma rules above, so a field
	// separator still wins: those decide the shape of `contexts: A, B` before a
	// brace ever gets a say. The one exception is a `}`, which those two rules
	// exclude explicitly so that the forced break before a closing brace is
	// never pre-empted by a trailing comma or colon.
	//
	// Two of the four are primary: break after `{` (`prev == LBrace`) and break
	// before `}` (`curr == RBrace`). The other two are their mirrors,
	// `curr == LBrace` here and `prev == RBrace` below. Without the mirrors a
	// minified declaration only half expanded: `service Foo{` kept the brace
	// glued to the name, and `}Commerce{` never broke at all.
	//
	// Both mirrors are guarded on the gap, for different reasons. Reading a
	// `\n` there as authorial is the walker's whole premise, so `service Foo\n{`
	// must keep the author's brace-on-its-own-line style rather than be pulled
	// up, and `}\n\n  Commerce {` must keep its blank line rather than be
	// flattened to a single newline. In both cases the newline handling below
	// already does the right thing, so the guard is simply "only act when the
	// author wrote no line break at all", which is exactly the minified case.
	//
	// The two mirrors are the whole of what minified expansion does. Several
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
		return "\n" + indentFor(depth, cfg)
	}
	if curr.Kind() == syntax.SyntaxKindRBrace {
		return "\n" + indentFor(depth, cfg)
	}

	// A scenario always gets a blank line before it, even if the author wrote
	// none. This is the one place the formatter adds vertical space rather than
	// preserving it, and it matches what the previous formatter did.
	//
	// The first scenario in a use_case body is not preceded by one. That case
	// never reaches here: it follows the `{`, and the `prev == LBrace` rule
	// above has already returned. This rule carried a `prev != LBrace` clause
	// that claimed to be what recognised it, which stopped being true when the
	// brace rules moved above it; deleting the clause changed no test.
	//
	// This sits ABOVE the `prev == RBrace` rule below, and the order is load
	// bearing. A `tags { }` block followed immediately by a `when` puts a `}`
	// and a `when` next to each other. With the brace rule first, a minified
	// `}when` got a plain newline on the first pass and the blank line only on
	// the second, once the newline was in the gap, so formatting was not
	// idempotent for that shape.
	if startsScenario && depth == 1 {
		return "\n\n" + indentFor(depth, cfg)
	}

	// The fourth brace rule, and the mirror of `curr == RBrace` above: a `}`
	// with something after it on the same line breaks, so `}Commerce{` expands.
	// See the block comment above the other three for why it is guarded on the
	// gap.
	//
	// It sits HERE, below the scenario rule, and not with the other three. That
	// is not tidiness, it is the fix for a non-idempotent shape: a `}` and a
	// `when` can end up adjacent, and if this rule got there first it emitted a
	// plain newline and the scenario's blank line only appeared on a second
	// pass. The scenario rule above carries the full account.
	if prev.Kind() == syntax.SyntaxKindRBrace && !strings.Contains(gap, "\n") {
		return "\n" + indentFor(depth, cfg)
	}

	// A continuation of a wrapped property value hangs one continuation unit
	// past the block indent, so it cannot be mistaken for a sibling statement.
	// Block indent, not visual indent aligned under the value: every formatter
	// that revisited this since 2016 moved the same way, because aligning under
	// the value re-wraps the whole list when the key is renamed. See
	// docs/decisions/formatting-configuration.md D4.
	//
	// The column is a pure function of depth. Deriving it from the emitted
	// column of the previous line would move it again whenever that line
	// shifted, which is a fixed-point failure and has bitten this file twice.
	contIndent := func() string {
		if !continuing {
			return indentFor(depth, cfg)
		}
		return indentFor(depth, cfg) + strings.Repeat(" ", cfg.ContinuationIndent)
	}

	switch strings.Count(gap, "\n") {
	case 0:
		if gap == "" {
			return ""
		}
		// Collapse any run of spaces or tabs to one.
		return " "
	case 1:
		return "\n" + contIndent()
	default:
		// Two or more newlines is a blank line. Collapse any longer run to one.
		return "\n\n" + contIndent()
	}
}

// rootGapSeparator returns the whitespace to emit before a root-level
// comment that has no declaration to attach to, such as one on the last
// line of the file.
//
// This is the same author-decides rule separatorFor's own newline-count
// switch applies within a declaration, deliberately reused here rather than
// a fixed blank line: two trailing comments the author wrote back to back
// (a single newline between them) must stay back to back, and a blank line
// the author put between them must survive as one. There is no depth to
// indent for: this only ever fires at brace depth zero.
//
// cfg is unused today: nothing here indents. It is threaded anyway so every
// site in the whitespace policy takes the same configuration, rather than
// this one function being the exception a future change to root-level
// spacing would have to notice and fix up.
func rootGapSeparator(gap string, cfg fmtconfig.Config) string {
	switch strings.Count(gap, "\n") {
	case 0:
		return " "
	case 1:
		return "\n"
	default:
		return "\n\n"
	}
}

// isRefColon reports whether tok is the `:` inside a node slug such as
// `bc:re/billing`, as opposed to a field separator such as `contexts:`.
func isRefColon(tok syntax.SyntaxToken) bool {
	parent := tok.Parent()
	return parent != nil && parent.Kind() == syntax.SyntaxKindRef
}
