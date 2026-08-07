package lsp

import (
	"strings"
	"unicode/utf8"
)

// alignAnnotations column-aligns trailing operation annotations.
//
// It runs over the formatter's output rather than over the tree, because
// alignment is the one decision that needs to see whole lines. It only ever
// rewrites the run of spaces before a `[`, so it cannot change content.
//
// A run is a stretch of consecutive lines that carry an annotation, plus any
// unannotated lines between them. A blank line ends a run. That matches the
// worked examples in docs/decisions/action-operation-brackets.md, where an
// unannotated action keeps the column rather than splitting it.
func alignAnnotations(s string) string {
	lines := strings.Split(s, "\n")

	runStart := -1
	flush := func(end int) {
		if runStart < 0 {
			return
		}
		col := 0
		for i := runStart; i < end; i++ {
			if body, _, ok := splitAnnotation(lines[i]); ok {
				if w := utf8.RuneCountInString(body); w+2 > col {
					col = w + 2
				}
			}
		}
		for i := runStart; i < end; i++ {
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
// A comment is never an annotation, however it ends. `// see note [1]` would
// otherwise be treated as an annotated line, and because it is usually the
// longest line in the run it would push every real annotation out to match its
// width. writeAlignedActions, which this pass replaced, excluded comment lines
// from the column for exactly that reason, and dropping the exclusion would
// have been a silent regression.
func splitAnnotation(line string) (body, ann string, ok bool) {
	trimmed := strings.TrimRight(line, " \t")
	if !strings.HasSuffix(trimmed, "]") {
		return "", "", false
	}

	// A comment on its own line, including the `*` continuation of a block
	// comment.
	switch lead := strings.TrimLeft(trimmed, " \t"); {
	case strings.HasPrefix(lead, "//"), strings.HasPrefix(lead, "/*"), strings.HasPrefix(lead, "*"):
		return "", "", false
	}

	open := strings.LastIndex(trimmed, "[")
	if open <= 0 {
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
	for i := 0; i+1 < open; i++ {
		if trimmed[i] != '/' || trimmed[i+1] != '/' {
			continue
		}
		if i == 0 || trimmed[i-1] == ' ' || trimmed[i-1] == '\t' {
			return "", "", false
		}
	}
	body = strings.TrimRight(trimmed[:open], " \t")
	if body == "" {
		return "", "", false
	}
	return body, trimmed[open:], true
}
