package lsp

import (
	"strings"
	"unicode/utf8"
)

// alignAnnotations column-aligns trailing operation annotations.
//
// It runs over the formatter's output rather than over the tree, because
// alignment is the one decision that needs to see whole lines. It only ever
// rewrites the run of spaces before a `[`, and never on a line it was told to
// leave alone, so it cannot change content.
//
// interior is the set of line indices that fall inside a single token's text,
// which writeTokens records as it emits. A line-oriented pass cannot see token
// boundaries: a multi-line block comment is ONE token carrying newlines, so
// splitting the document into lines hands this pass the comment's interior
// lines with nothing to distinguish them from real ones. An interior line
// ending in `]` therefore looked exactly like an annotated action, and got
// padded to the run's column, which rewrote whitespace inside a comment.
// Deriving the answer here instead, by tracking `/* */` nesting, would put a
// heuristic where the emit site has the exact answer.
//
// Lines in interior take no part in the pass at all: they neither start a run,
// nor end one, nor get rewritten. Treating them as invisible rather than as
// unannotated is what keeps a blank line inside a block comment from splitting
// an alignment run that the comment merely sits in the middle of.
//
// A run is a stretch of consecutive lines that carry an annotation, plus any
// unannotated lines between them. A blank line ends a run. That matches the
// worked examples in docs/decisions/action-operation-brackets.md, where an
// unannotated action keeps the column rather than splitting it.
func alignAnnotations(s string, interior map[int]bool) string {
	lines := strings.Split(s, "\n")

	runStart := -1
	flush := func(end int) {
		if runStart < 0 {
			return
		}
		col := 0
		for i := runStart; i < end; i++ {
			if interior[i] {
				continue
			}
			if body, _, ok := splitAnnotation(lines[i]); ok {
				if w := utf8.RuneCountInString(body); w+2 > col {
					col = w + 2
				}
			}
		}
		for i := runStart; i < end; i++ {
			if interior[i] {
				continue
			}
			body, ann, ok := splitAnnotation(lines[i])
			if !ok {
				continue
			}
			pad := col - utf8.RuneCountInString(body)
			if pad < 1 {
				pad = 1
			}
			lines[i] = body + strings.Repeat(" ", pad) + ann
		}
		runStart = -1
	}

	for i, line := range lines {
		if interior[i] {
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush(i)
			continue
		}
		if _, _, ok := splitAnnotation(line); ok && runStart < 0 {
			runStart = i
		}
	}
	flush(len(lines))

	return strings.Join(lines, "\n")
}

// splitAnnotation splits a line into the text before its trailing operation
// annotation and the annotation itself. ok is false when the line does not end
// in one.
//
// The boundary matches the parser's: the annotation is the last `[` on the line
// whose `]` is the line's final character. A `[` that does not close at end of
// line is prose, and this must leave it alone.
//
// A bracket inside comment text is never an annotation. `// see note [1]`
// would otherwise be treated as an annotated line, and because it is usually
// the longest line in the run it would push every real annotation out to match
// its width. writeAlignedActions, which this pass replaced, excluded comment
// lines from the column for exactly that reason, and dropping the exclusion
// would have been a silent regression.
//
// The question this can answer is only ever "which parts of THIS line are
// comment", which is why the guards below are about where a comment opens and
// where it closes on the line rather than about whether a comment is open at
// all. A line that a comment merely passes through carries no visible mark of
// what it is, so it is excluded upstream instead, by the interior set
// alignAnnotations is given.
func splitAnnotation(line string) (body, ann string, ok bool) {
	trimmed := strings.TrimRight(line, " \t")
	if !strings.HasSuffix(trimmed, "]") {
		return "", "", false
	}

	// A line that is nothing but comment, which includes the `*` continuation
	// of a block comment.
	//
	// A block comment that CLOSES on this line is not that: everything after
	// its `*/` is ordinary content, and an action written there is a real
	// action that has to align with its siblings. `*/ A asks B to c [GET /x]`
	// is the idiomatic shape and was being left unaligned. So a line opening
	// with a block comment disqualifies itself only when the comment does not
	// close on it, and when it does, the search for the annotation starts after
	// the close.
	//
	// That is safe rather than lucky: block comments do not nest, so a `*/` on
	// a line genuinely inside a comment body is impossible, and a body line
	// with no `*/` still returns false here whatever bracket it carries. A `//`
	// line comment runs to end of line by definition and can never have content
	// after it, so it keeps disqualifying the whole line.
	from := 0
	switch lead := strings.TrimLeft(trimmed, " \t"); {
	case strings.HasPrefix(lead, "//"):
		return "", "", false
	case strings.HasPrefix(lead, "/*"):
		// Search for the close AFTER the opening `/*`, so that the `*/` a
		// reader sees in `/*/ note [1]` is not mistaken for one: that line
		// opens a comment that is still open at its end.
		start := strings.Index(trimmed, "/*") + 2
		end := strings.Index(trimmed[start:], "*/")
		if end < 0 {
			return "", "", false
		}
		from = start + end + 2
	case strings.HasPrefix(lead, "*"):
		end := strings.Index(trimmed, "*/")
		if end < 0 {
			return "", "", false
		}
		from = end + 2
	}

	open := strings.LastIndex(trimmed, "[")
	if open <= 0 || open < from {
		return "", "", false
	}

	// A `[` sitting inside a trailing comment is comment text, not an
	// annotation, as in `A asks B for c // see [1]`.
	//
	// Only a `//` that actually opens a comment counts: one at the start of the
	// line or following whitespace. Testing the whole prefix for `//` was too
	// blunt, because the `//` in a URL follows a `:`, so
	// `A asks B for http://x/y [POST /v1/x]` lost its alignment. A `//` after
	// the bracket is inside the annotation and never reached here.
	//
	// The scan starts at from rather than at 0, so that a `//` sitting inside
	// the closed block comment on this line is not read as opening a line
	// comment over content that is in fact outside it.
	for i := from; i+1 < open; i++ {
		if trimmed[i] != '/' || trimmed[i+1] != '/' {
			continue
		}
		if i == from || trimmed[i-1] == ' ' || trimmed[i-1] == '\t' {
			return "", "", false
		}
	}
	body = strings.TrimRight(trimmed[:open], " \t")
	if body == "" {
		return "", "", false
	}
	return body, trimmed[open:], true
}
