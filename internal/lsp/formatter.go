package lsp

import (
	"strings"

	"github.com/tcarcao/craft/internal/syntax"
)

// FormatDocument formats a Craft DSL source string to canonical form:
//   - top-level declarations separated by a blank line
//   - block content indented 2 spaces per level
//   - colons: no space before, one space after
//   - commas: no space before, one space after
//   - use_case and arch blocks preserved verbatim
func FormatDocument(content string) string {
	if content == "" {
		return "\n"
	}
	gn, _, diags := syntax.Parse(content)
	if len(diags) > 0 {
		return content
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
		case syntax.SyntaxKindUseCaseDecl, syntax.SyntaxKindArchDecl:
			// Preserve original formatting for constructs with
			// semantic indentation (scenario / arch section lines).
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
