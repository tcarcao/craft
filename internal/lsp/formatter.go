package lsp

import (
	"log/slog"
	"strings"
	"unicode"

	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// FormatDocument formats a Craft DSL source string to canonical form:
//   - top-level declarations separated by a blank line
//   - block content indented 2 spaces per level
//   - colons: no space before, one space after
//   - commas: no space before, one space after
//   - use_case blocks formatted with 2-space when / 4-space actions / blank line between scenarios
//   - arch blocks preserved verbatim (free-form component chain syntax)
func FormatDocument(content string) string {
	out, _ := FormatDocumentChecked(content)
	return out
}

// FormatDocumentChecked is FormatDocument plus the reason it declined to
// format.
//
// FormatDocument returns its input unchanged when the parse produced a
// diagnostic too severe to re-render from, which a caller holding only the
// returned string cannot tell apart from "already formatted". `craft fmt`
// needs that distinction: silently leaving a broken file untouched, or
// reporting it as clean under --check, is worse than saying it was skipped.
//
// The second result is nil when the document was formatted, and otherwise the
// diagnostic that blocked it.
func FormatDocumentChecked(content string) (string, *craft.Diagnostic) {
	if content == "" {
		return "\n", nil
	}
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
						sb.WriteString(rootGapSeparator(gap))
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
			interior := writeTokens(&decl, node)
			sb.WriteString(alignAnnotations(decl.String(), interior))
		}
	}

	if !first {
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
// accepts without diagnostics, and left it load bearing for those it does not.
// One shape is known to reach it, and it is a typo rather than a formatter
// bug: an unterminated string at end of line yields an Ident whose Text() is
// the string's contents at the offset of the opening quote, with the leftover
// byte landing in a Whitespace token. The widths still sum, so the tree passes
// the losslessness check, but the token texts no longer reproduce the source
// and re-rendering from them therefore cannot either. That is a lexer defect
// upstream of the formatter and predates this branch; the message says nothing
// about a formatter bug because for the only known trigger it would be wrong.
//
// That trigger is fixed now: every token's text is sliced from source at its
// own range, so no known real input reaches this function with content that
// differs by more than whitespace. TestContentDrift_RefusesToLoseContent
// exercises it directly instead.
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
// It returns the set of written line indices that fall INSIDE a token rather
// than between two of them, which is the lines of a multi-line token strictly
// between its first and last. The last line is deliberately excluded: it is
// shared with whatever follows the token on that same physical line, so it is
// not claimed by the token. Only alignAnnotations needs this, and only because
// it is line oriented: a multi-line block comment is one token carrying
// newlines, so a pass that splits the output into lines cannot tell that
// pass's interior lines apart from real ones and will happily rewrite the
// whitespace in them. The walker is the one place that knows, exactly and
// without heuristics, where a token's text starts and stops, so it is the
// place that answers.
//
// Today only comments produce a multi-line token, but the set is kept in terms
// of tokens rather than of comments: the invariant the formatter rests on is
// that whitespace BETWEEN tokens is the only thing any pass may touch, and that
// holds whatever kind of token grows a newline next.
func writeTokens(sb *strings.Builder, node syntax.SyntaxNode) map[int]bool {
	braceDepth := 0
	scenarioDepth := 0
	gap := ""
	var prev *syntax.SyntaxToken
	prevLedScenario := false

	line := 0
	var interior map[int]bool

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

		sep := separatorFor(prev, gap, tok, braceDepth+scenarioDepth, startsScenario)
		sb.WriteString(sep)
		sb.WriteString(tok.Text())

		line += strings.Count(sep, "\n")
		if n := strings.Count(tok.Text(), "\n"); n > 0 {
			// Only the lines strictly between the token's first and last
			// emitted line are interior. The last line (line+n) is the
			// token's own last line, which it shares with whatever follows
			// it on that same physical line, so it must stay eligible for
			// alignment rather than being claimed by the token.
			if n > 1 {
				if interior == nil {
					interior = make(map[int]bool, n-1)
				}
				for k := 1; k < n; k++ {
					interior[line+k] = true
				}
			}
			line += n
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

	return interior
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
