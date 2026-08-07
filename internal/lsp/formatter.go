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

	for el := range root.ChildrenIter() {
		node, ok := el.(syntax.SyntaxNode)
		if !ok {
			// A comment with no declaration after it to attach to, such as one
			// on the last line of the file, is a direct child of the root.
			// Skipping every non-node child is how those were dropped.
			// Note this only fires for a comment the parser tokenised at root
			// level. A comment with nothing after it never gets that far: the
			// parser dumps everything past the last real token into a single
			// Whitespace token, so the text is there but the kind is not.
			// trailingCommentLines below is what recovers those.
			if tok, isTok := el.(syntax.SyntaxToken); isTok && isCommentKind(tok.Kind()) {
				if !first {
					sb.WriteString("\n\n")
				}
				first = false
				sb.WriteString(tok.Text())
			}
			continue
		}

		if !first {
			sb.WriteString("\n\n")
		}
		first = false

		switch node.Kind() {
		case syntax.SyntaxKindArchDecl:
			// Preserve original formatting: arch component chains use free-form
			// indentation that the formatter does not rewrite. writeTokens would
			// re-derive their indentation from brace depth, so arch must not
			// reach it.
			r := node.TextRange()
			sb.WriteString(strings.TrimSpace(content[r.Start:r.End]))
		default:
			writeTokens(&sb, node)
		}
	}

	if tail := trailingCommentLines(content, root); len(tail) > 0 {
		if !first {
			sb.WriteString("\n\n")
		}
		first = false
		sb.WriteString(strings.Join(tail, "\n"))
	}

	if !first {
		sb.WriteByte('\n')
	}

	formatted := alignAnnotations(sb.String())
	if drift := contentDrift(content, formatted); drift != nil {
		slog.Error("craft formatter: refusing to format, output would change content",
			"detail", drift.Message)
		return content, drift
	}
	return formatted, nil
}

// trailingCommentLines returns the comment lines that sit after the last real
// token in the document, one per line and stripped of indentation.
//
// A comment with a declaration after it is tokenised as trivia and attached to
// that declaration, so every renderer can find it. A comment with NOTHING
// after it never gets that treatment: the parser dumps the whole remainder of
// the source into one SyntaxKindWhitespace token, so the bytes survive in the
// tree (losslessness holds) but no token carries a comment kind. Filtering on
// kind therefore finds nothing, which is how a file ending in a comment lost
// it, and how a file that is nothing BUT a comment was truncated to zero
// bytes.
//
// Reaching the text through the trivia rather than through the kind is the
// fix. Nothing in this tail can be anything other than a comment or blank
// space: real content there would have produced a diagnostic, and the
// formatter would have declined to run at all.
func trailingCommentLines(content string, root syntax.SyntaxNode) []string {
	end := 0
	for _, tok := range root.AllTokens() {
		if tok.Kind() == syntax.SyntaxKindWhitespace || tok.Kind() == syntax.SyntaxKindEOF {
			continue
		}
		if e := int(tok.Offset()) + len(tok.Text()); e > end {
			end = e
		}
	}
	if end >= len(content) {
		return nil
	}

	var out []string
	for _, line := range strings.Split(content[end:], "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
func contentDrift(in, out string) *craft.Diagnostic {
	if squashWhitespace(in) == squashWhitespace(out) {
		return nil
	}
	return &craft.Diagnostic{
		Code:     "craft/internal/formatter-content-drift",
		Message:  "formatting would have changed more than whitespace, so the document was left unchanged; this is a formatter bug, please report the file",
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
func writeTokens(sb *strings.Builder, node syntax.SyntaxNode) {
	braceDepth := 0
	scenarioDepth := 0
	gap := ""
	var prev *syntax.SyntaxToken

	for _, tok := range node.AllTokens() {
		if tok.Kind() == syntax.SyntaxKindEOF {
			continue
		}
		if tok.Kind() == syntax.SyntaxKindWhitespace {
			gap += tok.Text()
			continue
		}

		// A `when` at the use_case's own brace depth closes any scenario body
		// that was open (it, and the enclosing `}` below, are the only two
		// things that end one) and sits at that same level itself; only the
		// lines after it are indented deeper. Resetting before the separator
		// is computed is what keeps the `when` line at depth 1 instead of 2.
		if tok.Kind() == syntax.SyntaxKindKwWhen && braceDepth == 1 {
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

		sb.WriteString(separatorFor(prev, gap, tok, braceDepth+scenarioDepth))
		sb.WriteString(tok.Text())

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
	}
}
