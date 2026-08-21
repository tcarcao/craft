package lsp

import (
	"log/slog"
	"strings"
	"unicode"

	"github.com/tcarcao/craft/v2/internal/fmtconfig"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// FormatDocument formats a Craft DSL source string to canonical form under
// the built-in defaults:
//   - top-level declarations separated by a blank line
//   - block content indented per fmtconfig.Defaults().Indent spaces per level
//   - colons: no space before, one space after
//   - commas: no space before, one space after
//   - use_case blocks formatted with when at scenario indent / actions one
//     level deeper / blank line between scenarios
//   - arch blocks preserved verbatim (free-form component chain syntax)
func FormatDocument(content string) string {
	return FormatDocumentWith(content, fmtconfig.Defaults())
}

// FormatDocumentWith is FormatDocument under an explicit configuration.
func FormatDocumentWith(content string, cfg fmtconfig.Config) string {
	out, _ := FormatDocumentCheckedWith(content, cfg)
	return out
}

// FormatDocumentChecked is FormatDocumentCheckedWith under the built-in
// defaults.
func FormatDocumentChecked(content string) (string, *craft.Diagnostic) {
	return FormatDocumentCheckedWith(content, fmtconfig.Defaults())
}

// FormatDocumentCheckedWith is FormatDocumentWith plus the reason it declined
// to format.
//
// FormatDocumentWith returns its input unchanged when the parse produced a
// diagnostic too severe to re-render from, which a caller holding only the
// returned string cannot tell apart from "already formatted". `craft fmt`
// needs that distinction: silently leaving a broken file untouched, or
// reporting it as clean under --check, is worse than saying it was skipped.
//
// The second result is nil when the document was formatted, and otherwise the
// diagnostic that blocked it.
func FormatDocumentCheckedWith(content string, cfg fmtconfig.Config) (string, *craft.Diagnostic) {
	gn, _, diags := syntax.Parse(content)
	for _, d := range diags {
		if bailsFormatting(d) {
			return content, &d
		}
	}
	root := syntax.Root(gn)

	var sb strings.Builder
	first := true
	gap := ""

	for el := range root.ChildrenIter() {
		node, ok := el.(syntax.SyntaxNode)
		if !ok {
			// A comment with no declaration after it to attach to, such as one
			// on the last line of the file, is a direct child of the root
			// rather than trivia attached inside a declaration node. Since
			// docs/decisions/lossless-token-text.md, the parser tokenises
			// these with their own comment kind instead of folding them into
			// trailing whitespace, so they reach here already carrying the
			// kind check below needs.
			if tok, isTok := el.(syntax.SyntaxToken); isTok {
				if tok.Kind() == syntax.SyntaxKindWhitespace {
					gap += tok.Text()
					continue
				}
				if isCommentKind(tok.Kind()) {
					if !first {
						sb.WriteString(rootGapSeparator(gap, cfg))
					}
					first = false
					sb.WriteString(tok.Text())
				}
			}
			gap = ""
			continue
		}

		if !first {
			sb.WriteString("\n\n")
		}
		first = false
		gap = ""

		switch node.Kind() {
		case syntax.SyntaxKindArchDecl:
			// Preserve original formatting: arch component chains use free-form
			// indentation that the formatter does not rewrite. writeTokens would
			// re-derive their indentation from brace depth, so arch must not
			// reach it.
			r := node.TextRange()
			sb.WriteString(strings.TrimSpace(content[r.Start:r.End]))
		default:
			// Alignment runs per declaration, on the walker's own output, and
			// never over the assembled document. Run over the document it would
			// also rewrite the verbatim arch slice above, where `WebApp[ssl,
			// cache]` is a modifier list rather than an operation annotation:
			// splitAnnotation cannot tell the two apart, so it would pad
			// `WebApp` out to a column and break the verbatim guarantee.
			//
			// Per declaration is not a weaker pass. An alignment run is bounded
			// by blank lines, and top-level declarations are always joined by
			// one, so no run could ever have spanned two declarations anyway.
			var decl strings.Builder
			interior, commentEnd := writeTokens(&decl, node, cfg)
			sb.WriteString(alignAnnotations(decl.String(), interior, commentEnd))
		}
	}

	// The document ends in exactly one newline. Most declarations end on a
	// token that carries none, so one is added here. An unterminated block
	// comment is the exception: the lexer slices it to EOF, so its own text
	// already ends in the file's final newline, and it reaches this loop as a
	// root-level comment token written verbatim. Adding a second newline made
	// formatting non-idempotent, and under format-on-save the file grew a
	// blank line on every keystroke-triggered save, without bound.
	//
	// Asking what was already written, rather than trimming the token, is what
	// keeps this stage on the right side of the token-text-is-sacred rule: a
	// pass that rewrites token text is the hazard this whole design removes.
	if !first && !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteByte('\n')
	}

	formatted := sb.String()
	if drift := contentDrift(content, formatted); drift != nil {
		// Warn rather than Error, and without asking for a bug report. The one
		// shape known to reach here is a user typo (see contentDrift), and
		// telling someone who left a string unterminated that they have found a
		// formatter bug sends them to the wrong place.
		slog.Warn("craft formatter: declining to format, output would not have matched the source",
			"detail", drift.Message)
		return content, drift
	}
	return formatted, nil
}

// contentDrift enforces the formatter's contract: formatting changes
// whitespace and nothing else. It returns nil when every non-whitespace byte
// of in appears in out in the same order, and otherwise the diagnostic
// explaining the refusal.
//
// This is a structural guarantee, not another bug fix. FormatDocument
// reconstructs each declaration from typed accessors, so every construct needs
// its own branch and any construct without one is dropped in silence. Eight
// defects of exactly that shape were found on this branch, the last of them a
// comment position nobody had thought of. Adding branches makes that design
// less unsafe, never safe. This check makes the failure mode harmless instead:
// a construct with no branch turns a silent deletion into a no-op that says so.
//
// It stays even once every known case is fixed, because the branch it protects
// against is the one nobody has written yet.
//
// The token-stream rewrite made it unreachable for any document the parser
// accepts without diagnostics. One shape used to reach it anyway, and it was a
// typo rather than a formatter bug: an unterminated string at end of line
// yielded a token whose Text() was the string's contents at the offset of the
// opening quote, with the leftover byte landing in a Whitespace token. The
// widths still summed, so the tree passed the losslessness check, but the token
// texts no longer reproduced the source. The message says nothing about a
// formatter bug because for that trigger it would have been wrong.
//
// That was a parser defect, and it is fixed: every token's text is now sliced
// from source at its own range and checkTreeText asserts the tree reproduces
// its file, so no input reaches this function with content differing by more
// than whitespace. What it still guards is a bug in the walker itself dropping,
// duplicating or reordering a token, which no upstream invariant can rule out.
// TestContentDrift_RefusesToLoseContent exercises it directly, since nothing
// reaches it end to end.
//
// Note what it cannot catch: it compares whitespace-squashed strings, so it is
// blind to whitespace defects by construction. Both whitespace bugs found on
// this branch, an unterminated block comment growing a newline per save and a
// comment body padded to an alignment column, passed it untouched.
func contentDrift(in, out string) *craft.Diagnostic {
	if squashWhitespace(in) == squashWhitespace(out) {
		return nil
	}
	return &craft.Diagnostic{
		Code:     "craft/internal/formatter-content-drift",
		Message:  "formatting would have changed more than whitespace, so the document was left unchanged; check the file for an unterminated string or another typo the parser did not flag",
		Severity: craft.SeverityWarning,
	}
}

// squashWhitespace returns s with every whitespace character removed, so two
// strings compare equal exactly when they carry the same content bytes in the
// same order.
func squashWhitespace(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// bailsFormatting reports whether a syntax diagnostic means the tree is too
// incomplete to rewrite safely, so FormatDocument must return the input
// untouched.
//
// Errors are the obvious case. `craft/syntax/not-yet-implemented` is included
// because it is only warning-severity yet says precisely that the parser could
// not place a construct: the tokens survive in the tree as raw leaves but the
// structure around them is wrong, and re-rendering from that structure drops
// or duplicates them. That is how `charge {amount} [POST /pay]` came back as
// `charge {amount  [ POST / pay ]`.
func bailsFormatting(d craft.Diagnostic) bool {
	return d.Severity == craft.SeverityError || d.Code == "craft/syntax/not-yet-implemented"
}

// isCommentKind reports whether a leaf token is a comment. Line and block
// comments are trivia and excluded from Tokens(); doc comments are not trivia
// and are included. Every renderer has to treat all three the same way, so the
// test lives here rather than being spelled out at each site.
func isCommentKind(k syntax.SyntaxKind) bool {
	return k == syntax.SyntaxKindLineComment ||
		k == syntax.SyntaxKindBlockComment ||
		k == syntax.SyntaxKindDocComment
}

// writeTokens renders one top-level declaration by walking its token stream.
//
// Every non-whitespace token is written verbatim, exactly once, in document
// order. The only decision is the separator before each, which separatorFor
// makes. There are deliberately no per-construct branches: a construct the
// formatter has never seen still round-trips, because nothing here inspects
// what a token means.
//
// It returns two pieces of bookkeeping, both for alignAnnotations, and both
// for the same reason: that pass is line oriented, so it sees the walker's
// output only as text and cannot recover where a token began or ended. The
// walker is the one place that knows both exactly, so it is the place that
// answers, rather than leaving a downstream pass to guess from the shape of a
// line.
//
// interior is the set of written line indices that fall INSIDE a token rather
// than between two of them: every line a multi-line token spans EXCEPT its
// last. The rule is "the token's text runs to the end of this line". That is
// true of the token's first line (the token starts partway along it and then
// runs on past the newline) and of every line in between, and false only of
// the last, which the token stops partway along and shares with whatever
// follows the close. Since an annotation is by definition the end of its line,
// a line the token runs off the end of can never carry one, so excluding it
// only ever removes a false positive.
//
// Today only comments produce a multi-line token, but interior is kept in terms
// of tokens rather than of comments: the invariant the formatter rests on is
// that whitespace BETWEEN tokens is the only thing any pass may touch, and that
// holds whatever kind of token grows a newline next.
//
// commentEnd maps a written line index to the BYTE offset within that line just
// past the end of the last comment token that ENDS on it. Lines with no comment
// ending on them are absent, which reads as zero: nothing on the line is
// comment, so an annotation may sit anywhere. A `[` at or after the recorded
// offset is outside every comment on the line and can be a real annotation; one
// before it is comment text. Byte offsets rather than rune counts, because
// splitAnnotation compares against strings.LastIndex, which is a byte index.
//
// A line comment runs to the end of its line by definition, so its offset is
// the line's length and every bracket on the line falls before it. A block
// comment closing partway along a line yields the offset just past its `*/`,
// which is exactly where ordinary content resumes.
func writeTokens(sb *strings.Builder, node syntax.SyntaxNode, cfg fmtconfig.Config) (map[int]bool, map[int]int) {
	braceDepth := 0
	scenarioDepth := 0
	gap := ""
	var prev *syntax.SyntaxToken
	prevLedScenario := false

	line := 0
	var interior map[int]bool
	var commentEnd map[int]int

	// col is the byte length of the output line currently being built, over
	// everything this walker has written. It is tracked incrementally rather
	// than measured from the builder, because strings.Builder cannot be read
	// without materialising the whole declaration on every token.
	col := 0
	advance := func(s string) {
		if n := strings.LastIndexByte(s, '\n'); n >= 0 {
			col = len(s) - n - 1
			return
		}
		col += len(s)
	}

	toks := node.AllTokens()
	for i, tok := range toks {
		if tok.Kind() == syntax.SyntaxKindEOF {
			continue
		}
		if tok.Kind() == syntax.SyntaxKindWhitespace {
			gap += tok.Text()
			continue
		}

		// A comment directly above a `when` documents the scenario below it,
		// not the one above it, so it belongs to the coming scenario and has to
		// close the open one just as the `when` itself does. Without this the
		// comment kept the previous scenario's body depth, so it was written at
		// action indent instead of scenario indent, and the blank line landed
		// between the comment and its `when` rather than above the pair.
		//
		// This needs lookahead, which is why the walker answers it rather than
		// separatorFor: a comment's owner is the next real token, and
		// separatorFor only ever sees one token at a time.
		//
		// The comment must also start its own line. A TRAILING comment belongs
		// to the action it sits on, however close the next `when` is: without
		// that clause `P does y  // note` had its comment lifted off the action,
		// re-indented from action level to scenario level and given a blank
		// line, which is the same comment re-indentation this lookahead exists
		// to remove.
		startsOwnLine := prev == nil || strings.Contains(gap, "\n")
		leadsScenario := isCommentKind(tok.Kind()) && braceDepth == 1 &&
			startsOwnLine && nextRealTokenIsWhen(toks, i)

		// A `when` at the use_case's own brace depth closes any scenario body
		// that was open (it, the leading comment above, and the enclosing `}`
		// below are the only things that end one) and sits at that same level
		// itself; only the lines after it are indented deeper. Resetting before
		// the separator is computed is what keeps the `when` line at depth 1
		// instead of 2.
		startsWhen := tok.Kind() == syntax.SyntaxKindKwWhen && braceDepth == 1
		if startsWhen || leadsScenario {
			scenarioDepth = 0
		}

		// `}` closes its block, so it dedents before it is placed, and closes
		// any open scenario along with it.
		if tok.Kind() == syntax.SyntaxKindRBrace {
			scenarioDepth = 0
			if braceDepth > 0 {
				braceDepth--
			}
		}

		// The scenario's blank line goes above the whole run, so only the first
		// token of it asks for one. A `when` whose comment already opened the
		// scenario must not ask for a second.
		startsScenario := (startsWhen || leadsScenario) && !prevLedScenario

		sep := separatorFor(prev, gap, tok, braceDepth+scenarioDepth, startsScenario, cfg)
		sb.WriteString(sep)
		sb.WriteString(tok.Text())

		line += strings.Count(sep, "\n")
		advance(sep)
		if n := strings.Count(tok.Text(), "\n"); n > 0 {
			// Every line this token runs off the end of is interior: its own
			// first line (line) and each line in between. Only the last line
			// (line+n) is excluded, because the token stops partway along it
			// and shares it with whatever follows the close, which must stay
			// eligible for alignment rather than being claimed by the token.
			//
			// Claiming the first line matters as much as releasing the last.
			// A comment opened mid-line, as in `A asks B to c /* note [1]`,
			// leaves a line whose visible tail is comment body but which shows
			// the alignment pass nothing to say so, and the pass padded that
			// `[1]` out to the surrounding column: whitespace rewritten inside
			// a comment. The token's own extent is the only reliable answer.
			if interior == nil {
				interior = make(map[int]bool, n)
			}
			for k := 0; k < n; k++ {
				interior[line+k] = true
			}
			line += n
		}
		advance(tok.Text())

		// Where the comment stops is recorded for the line it stops ON, which
		// for a multi-line comment is its last: line has already been advanced
		// past the newlines the token carries. Recording it for every comment
		// kind, single-line and multi-line alike, is what lets the alignment
		// pass drop its own reading of what a line looks like. Later comments
		// on the same line overwrite earlier ones, so what survives is the last
		// one to end there, which is the only one that can shadow a `[`.
		if isCommentKind(tok.Kind()) {
			if commentEnd == nil {
				commentEnd = make(map[int]int, 1)
			}
			commentEnd[line] = col
		}

		if tok.Kind() == syntax.SyntaxKindLBrace {
			braceDepth++
		}

		// The `when` line itself stays at the block's own level; only the
		// lines after it (up to the next `when` at this depth, or the
		// enclosing `}`) are one level deeper. Bumping after the `when`
		// token has already been written is what keeps the bump from
		// applying to the `when` line itself.
		if tok.Kind() == syntax.SyntaxKindKwWhen && braceDepth == 1 {
			scenarioDepth = 1
		}

		cur := tok
		prev = &cur
		gap = ""
		prevLedScenario = leadsScenario
	}

	return interior, commentEnd
}

// nextRealTokenIsWhen reports whether the first token after i that carries
// meaning, skipping whitespace and any further comments, is a `when`.
//
// Skipping comments is what makes a run of them work: every comment in
// `// a`, `// b`, `when ...` sees the same `when` and the whole run attaches to
// that scenario, rather than only the last one.
func nextRealTokenIsWhen(toks []syntax.SyntaxToken, i int) bool {
	for j := i + 1; j < len(toks); j++ {
		k := toks[j].Kind()
		if k == syntax.SyntaxKindWhitespace || k == syntax.SyntaxKindEOF || isCommentKind(k) {
			continue
		}
		return k == syntax.SyntaxKindKwWhen
	}
	return false
}
