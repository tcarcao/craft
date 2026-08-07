package lsp

import (
	"strings"
	"unicode/utf8"

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
	if content == "" {
		return "\n"
	}
	gn, _, diags := syntax.Parse(content)
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			return content
		}
	}
	root := syntax.Root(gn)

	var sb strings.Builder
	first := true

	for el := range root.ChildrenIter() {
		node, ok := el.(syntax.SyntaxNode)
		if !ok {
			continue
		}

		if !first {
			sb.WriteString("\n\n")
		}
		first = false

		switch node.Kind() {
		case syntax.SyntaxKindUseCaseDecl:
			formatUseCaseDecl(&sb, node)
		case syntax.SyntaxKindArchDecl:
			// Preserve original formatting: arch component chains use
			// free-form indentation that the formatter does not rewrite.
			r := node.TextRange()
			sb.WriteString(strings.TrimSpace(content[r.Start:r.End]))
		default:
			formatDecl(&sb, node)
		}
	}

	if !first {
		sb.WriteByte('\n')
	}
	return sb.String()
}

// formatUseCaseDecl formats a use_case block with canonical indentation:
// each `when` scenario at 2-space indent, actions at 4-space indent,
// blank line between scenarios.
func formatUseCaseDecl(sb *strings.Builder, node syntax.SyntaxNode) {
	uc := syntax.AsUseCaseDecl(node)

	sb.WriteString("use_case")
	if title := uc.Title(); title != nil {
		// title.Text() is the raw source text and already includes both
		// quotes (Bug 8a fix) — do not wrap it again.
		sb.WriteByte(' ')
		sb.WriteString(title.Text())
	}
	sb.WriteString(" {\n")

	for i, sc := range uc.Scenarios() {
		if i > 0 {
			sb.WriteByte('\n') // blank line between scenarios
		}
		// Both renderers are the source-faithful ones. Anything that rebuilds
		// a line by space-joining leaf tokens silently rewrites the user's
		// file: it splits a qualified ref into "re / billing" and pulls a
		// phrase like "(1! & 2!)" apart into "( 1 ! & 2 ! )".
		sb.WriteString("  when ")
		sb.WriteString(sc.Trigger().SourceText())
		sb.WriteByte('\n')
		writeAlignedActions(sb, sc.Actions())
	}

	sb.WriteByte('}')
}

// writeAlignedActions writes a scenario's action lines, column-aligning any
// trailing `[...]` operation annotations to a shared column.
//
// A scenario's action list is the whole alignment run: nothing but actions
// ever appears between them (a blank line or `when` only occurs at a scenario
// boundary, which is a separate call to this function). The column is
// max(visible length of the line before the annotation) + 2, computed over
// only the annotated lines. An action with no annotation contributes nothing
// to that max and is written back exactly as SourceText renders it, with no
// trailing padding.
func writeAlignedActions(sb *strings.Builder, actions []syntax.ActionDecl) {
	type actionLine struct {
		body string // SourceText with the " [op]" suffix stripped
		op   string // annotation body without brackets, "" when none
	}

	lines := make([]actionLine, len(actions))
	col := 0
	for i, action := range actions {
		full := action.SourceText()
		op := action.OpText()
		body := full
		if op != "" {
			body = strings.TrimSuffix(full, " ["+op+"]")
			if l := utf8.RuneCountInString(body); l+2 > col {
				col = l + 2
			}
		}
		lines[i] = actionLine{body: body, op: op}
	}

	for _, l := range lines {
		sb.WriteString("    ")
		sb.WriteString(l.body)
		if l.op != "" {
			pad := col - utf8.RuneCountInString(l.body)
			sb.WriteString(strings.Repeat(" ", pad))
			sb.WriteByte('[')
			sb.WriteString(l.op)
			sb.WriteByte(']')
		}
		sb.WriteByte('\n')
	}
}

// formatDecl formats a single top-level declaration into sb.
func formatDecl(sb *strings.Builder, node syntax.SyntaxNode) {
	tokens := node.Tokens()
	if len(tokens) == 0 {
		return
	}

	depth := 0
	needsNewline := false

	for i, tok := range tokens {
		kind := tok.Kind()

		switch kind {
		case syntax.SyntaxKindLBrace:
			sb.WriteString(" {")
			depth++
			needsNewline = true

		case syntax.SyntaxKindRBrace:
			depth--
			sb.WriteByte('\n')
			sb.WriteString(strings.Repeat("  ", depth))
			sb.WriteByte('}')
			needsNewline = depth > 0

		case syntax.SyntaxKindColon:
			sb.WriteByte(':')

		case syntax.SyntaxKindComma:
			sb.WriteByte(',')

		case syntax.SyntaxKindString:
			// tok.Text() is the exact raw source text, already including both
			// quotes (Bug 8a fix) — do not wrap it again.
			sep := tokenSeparator(tokens, i, depth, needsNewline)
			needsNewline = false
			sb.WriteString(sep)
			sb.WriteString(tok.Text())

		default:
			sep := tokenSeparator(tokens, i, depth, needsNewline)
			needsNewline = false
			sb.WriteString(sep)
			sb.WriteString(tok.Text())
		}
	}
}

// tokenSeparator determines the whitespace string to emit before tokens[i].
func tokenSeparator(tokens []syntax.SyntaxToken, i, depth int, needsNewline bool) string {
	if i == 0 {
		return ""
	}
	if needsNewline {
		return "\n" + strings.Repeat("  ", depth)
	}

	prev := tokens[i-1]
	curr := tokens[i]
	prevKind := prev.Kind()

	// After : or , → space (value continuation)
	if prevKind == syntax.SyntaxKindColon || prevKind == syntax.SyntaxKindComma {
		return " "
	}
	// Before : or , → no space
	if curr.Kind() == syntax.SyntaxKindColon || curr.Kind() == syntax.SyntaxKindComma {
		return ""
	}

	if depth == 0 {
		// At top level, tokens within the same declaration are space-separated.
		// isNextColon and isSiblingToken only apply inside blocks (depth > 0).
		return " "
	}

	// Field key: if the NEXT token is `:`, this is a field name → new line
	if isNextColon(tokens, i) {
		return "\n" + strings.Repeat("  ", depth)
	}

	// Different sibling nodes under the same parent → new line
	if isSiblingToken(prev, curr) {
		return "\n" + strings.Repeat("  ", depth)
	}

	return " "
}

// isNextColon returns true if tokens[i+1] is a colon.
func isNextColon(tokens []syntax.SyntaxToken, i int) bool {
	return i+1 < len(tokens) && tokens[i+1].Kind() == syntax.SyntaxKindColon
}

// isSiblingToken returns true when prev and curr belong to different child
// nodes of the same parent block node.
func isSiblingToken(prev, curr syntax.SyntaxToken) bool {
	pp := prev.Parent()
	cp := curr.Parent()
	if pp == nil || cp == nil {
		return false
	}
	if pp.Equal(*cp) {
		return false
	}
	gp := pp.Parent()
	gc := cp.Parent()
	if gp == nil || gc == nil {
		return false
	}
	return gp.Equal(*gc)
}
