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
// Both maps come from writeTokens, and both exist because this pass sees only
// text. Where a token started and where it stopped is knowledge the walker had
// exactly and this pass cannot recover, so it is handed down rather than
// re-derived. Nothing here reads the shape of a line to decide what it is.
//
// interior is the set of line indices that fall inside a single token's text.
// A line-oriented pass cannot see token boundaries: a multi-line block comment
// is ONE token carrying newlines, so splitting the document into lines hands
// this pass the comment's interior lines with nothing to distinguish them from
// real ones. An interior line ending in `]` therefore looked exactly like an
// annotated action, and got padded to the run's column, which rewrote
// whitespace inside a comment.
//
// Lines in interior take no part in the pass at all: they neither start a run,
// nor end one, nor get rewritten. Treating them as invisible rather than as
// unannotated is what keeps a blank line inside a block comment from splitting
// an alignment run that the comment merely sits in the middle of.
//
// commentEnd gives, per line, the byte offset just past the last comment that
// ends on it, and is what splitAnnotation needs to tell an annotation from a
// bracket in comment text. A line absent from it has no comment ending on it,
// and zero is the right answer for such a line.
//
// A run is a stretch of consecutive lines that carry an annotation, plus any
// unannotated lines between them. A blank line ends a run. That matches the
// worked examples in docs/decisions/action-operation-brackets.md, where an
// unannotated action keeps the column rather than splitting it.
func alignAnnotations(s string, interior map[int]bool, commentEnd map[int]int) string {
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
			if body, _, ok := splitAnnotation(lines[i], commentEnd[i]); ok {
				if w := utf8.RuneCountInString(body); w+2 > col {
					col = w + 2
				}
			}
		}
		for i := runStart; i < end; i++ {
			if interior[i] {
				continue
			}
			body, ann, ok := splitAnnotation(lines[i], commentEnd[i])
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
		if _, _, ok := splitAnnotation(line, commentEnd[i]); ok && runStart < 0 {
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
// from says where comment text on this line stops: the byte offset just past
// the last comment token ending on it, as recorded by writeTokens. This
// function does not work that out for itself, and that is the whole point. It
// used to, by pattern-matching leading `//`, `/*` and `*` and then scanning for
// a `//` that looked like it opened a comment, and every alignment defect left
// on this pass was a spelling those patterns did not cover. A comment closing
// on a line that begins with a word matched no case, so from stayed zero and
// the scan then found the `//` inside `see // here */ A asks B to c [GET /x]`
// and refused the line; the same comment written with a leading `*` aligned.
// Same content, different spelling, different answer. The lexer already knew
// where that comment ended.
//
// The direction that matters is from being too SMALL: comment text would then
// be read as an annotation and the whitespace inside a comment rewritten, which
// is content corruption rather than a missed alignment. A from that is exactly
// right cannot do that.
//
// A line a comment merely passes through has no comment ENDING on it, so from
// says nothing useful about it. Those lines are excluded upstream instead, by
// the interior set alignAnnotations is given.
func splitAnnotation(line string, from int) (body, ann string, ok bool) {
	trimmed := strings.TrimRight(line, " \t")
	if !strings.HasSuffix(trimmed, "]") {
		return "", "", false
	}

	// open < from puts the bracket inside comment text. That covers a line
	// comment, whose token runs to end of line so that from is past every
	// bracket on it, and a block comment closing partway along, where only the
	// brackets before the `*/` are shadowed.
	open := strings.LastIndex(trimmed, "[")
	if open <= 0 || open < from {
		return "", "", false
	}

	body = strings.TrimRight(trimmed[:open], " \t")
	if body == "" {
		return "", "", false
	}
	return body, trimmed[open:], true
}

// splitTrailingComment splits a line into the text before its trailing comment
// and the comment itself. ok is false when the line carries no trailing comment
// cell.
//
// start is the byte offset where the comment token begins, as recorded by
// writeTokens. Like splitAnnotation's `from`, this function does not work it
// out for itself, and for the same reason: pattern-matching `//` cannot tell a
// comment from the `//` inside a narrative phrase such as
// `A asks B for http://x`, and the lexer already knew. A start of zero means
// the walker recorded no trailing comment for this line.
//
// A comment-only line yields an empty body and ok false. Aligning those would
// push a whole run out to match a full-width comment, and a comment on its own
// line is not a cell in the same column as one that follows content.
func splitTrailingComment(line string, start int) (body, cell string, ok bool) {
	if start <= 0 || start > len(line) {
		return "", "", false
	}
	body = strings.TrimRight(line[:start], " \t")
	if body == "" {
		return "", "", false
	}
	cell = strings.TrimRight(line[start:], " \t")
	if cell == "" {
		return "", "", false
	}
	return body, cell, true
}
