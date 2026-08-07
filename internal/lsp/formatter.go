package lsp

import (
	"log/slog"
	"strings"
	"unicode"
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
		case syntax.SyntaxKindUseCaseDecl:
			formatUseCaseDecl(&sb, node)
		case syntax.SyntaxKindContextMapDecl:
			formatContextMapDecl(&sb, node)
		case syntax.SyntaxKindGlossaryDecl:
			formatGlossaryDecl(&sb, node)
		case syntax.SyntaxKindArchDecl:
			// Preserve original formatting: arch component chains use
			// free-form indentation that the formatter does not rewrite.
			r := node.TextRange()
			sb.WriteString(strings.TrimSpace(content[r.Start:r.End]))
		default:
			formatDecl(&sb, node)
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

	formatted := sb.String()
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

// formatUseCaseDecl formats a use_case block with canonical indentation:
// each `when` scenario at 2-space indent, actions at 4-space indent,
// blank line between scenarios.
func formatUseCaseDecl(sb *strings.Builder, node syntax.SyntaxNode) {
	uc := syntax.AsUseCaseDecl(node)

	writeCommentLines(sb, "", uc.LeadingComments())
	sb.WriteString("use_case")
	if title := uc.Title(); title != nil {
		// title.Text() is the raw source text and already includes both
		// quotes (Bug 8a fix) — do not wrap it again.
		sb.WriteByte(' ')
		sb.WriteString(title.Text())
	}
	sb.WriteString(" {\n")

	// A tags { } block is a child of the use case, not of any scenario, so a
	// renderer that walks only Scenarios() drops it silently. That is what
	// deleted every `tags` block, along with the qualified refs inside it such
	// as `journey: re/renewal-flow`.
	tagBlocks := uc.TagsBlocks()
	for _, tags := range tagBlocks {
		writeCommentLines(sb, "  ", syntax.LeadingComments(tags.Node()))
		sb.WriteString("  tags {\n")
		writeBlockStatements(sb, tags.Node(), syntax.SyntaxKindTagStmt, "    ")
		sb.WriteString("  }\n")
	}

	for i, sc := range uc.Scenarios() {
		if i > 0 || len(tagBlocks) > 0 {
			sb.WriteByte('\n') // blank line between scenarios
		}
		writeCommentLines(sb, "  ", sc.LeadingComments())
		// Both renderers are the source-faithful ones. Anything that rebuilds
		// a line by space-joining leaf tokens silently rewrites the user's
		// file: it splits a qualified ref into "re / billing" and pulls a
		// phrase like "(1! & 2!)" apart into "( 1 ! & 2 ! )".
		sb.WriteString("  when ")
		sb.WriteString(sc.Trigger().SourceText())
		sb.WriteByte('\n')
		writeAlignedActions(sb, sc.Actions())
		writeCommentLines(sb, "    ", sc.TrailingComments())
	}

	// A comment sitting between the last action and the closing brace reads as
	// part of that scenario's body, so it keeps action indent. With no
	// scenario at all there is no body to belong to and it sits at block level.
	trailingIndent := "  "
	if len(uc.Scenarios()) > 0 {
		trailingIndent = "    "
	}
	writeCommentLines(sb, trailingIndent, uc.TrailingComments())
	sb.WriteByte('}')
}

// writeCommentLines writes each comment on its own line at the given indent.
// A comment always owns its line: keeping a trailing comment inline would
// require re-deriving which line it belonged to after every other line has
// been re-indented, and getting that wrong silently comments out real content.
func writeCommentLines(sb *strings.Builder, indent string, comments []string) {
	for _, comment := range comments {
		sb.WriteString(indent)
		sb.WriteString(comment)
		sb.WriteByte('\n')
	}
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
		comments []string // comment lines written above this action
		body     string   // the action rendered without its " [op]" suffix
		op       string   // annotation body without brackets, "" when none
	}

	lines := make([]actionLine, len(actions))
	col := 0
	for i, action := range actions {
		// SourceTextWithoutOp rather than trimming the suffix off SourceText:
		// a trim has to spell " ["+op+"]" exactly as SourceText builds it, and
		// a no-op trim would emit the annotation twice.
		body := action.SourceTextWithoutOp()
		op := action.OpText()
		if op != "" {
			if l := utf8.RuneCountInString(body); l+2 > col {
				col = l + 2
			}
		}
		// A comment line takes no part in the alignment column: it is not an
		// annotated action, and letting a long comment push the column out
		// would be surprising. It does not end the run either, matching the
		// existing rule that only a blank line or a new scenario does.
		lines[i] = actionLine{comments: action.LeadingComments(), body: body, op: op}
	}

	for _, l := range lines {
		for _, comment := range l.comments {
			sb.WriteString("    ")
			sb.WriteString(comment)
			sb.WriteByte('\n')
		}
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

// formatContextMapDecl formats a context_map block: the header, then one edge
// statement per line at 2-space indent.
//
// It exists because formatDecl cannot render these blocks without corrupting
// them, in two independent ways. A qualified endpoint is a SyntaxKindRef node
// whose `/` is a separate leaf token, so space-joining produces
// `re / billing`, which no longer parses. And two adjacent edge statements are
// siblings whose token parents are different Ref nodes under different
// EdgeStmt parents, so isSiblingToken's same-grandparent test says "not
// siblings" and the whole block collapses onto one line.
//
// Both are solved by the seam the use_case renderers already use: ask the node
// for its own source text rather than reassembling it from leaves.
func formatContextMapDecl(sb *strings.Builder, node syntax.SyntaxNode) {
	cm := syntax.AsContextMapDecl(node)
	writeCommentLines(sb, "", syntax.LeadingComments(node))
	sb.WriteString("context_map")
	if domain := cm.Domain(); domain != "" {
		sb.WriteByte(' ')
		sb.WriteString(domain)
	}
	sb.WriteString(" {\n")
	writeBlockStatements(sb, node, syntax.SyntaxKindEdgeStmt, "  ")
	sb.WriteByte('}')
}

// formatGlossaryDecl formats a glossary block. Structurally identical to
// formatContextMapDecl, and corrupted by formatDecl for the same two reasons,
// with the extra sting that a three-segment term node carries two slashes.
func formatGlossaryDecl(sb *strings.Builder, node syntax.SyntaxNode) {
	gl := syntax.AsGlossaryDecl(node)
	writeCommentLines(sb, "", syntax.LeadingComments(node))
	sb.WriteString("glossary")
	if domain := gl.Domain(); domain != "" {
		sb.WriteByte(' ')
		sb.WriteString(domain)
	}
	sb.WriteString(" {\n")
	writeBlockStatements(sb, node, syntax.SyntaxKindGlossaryRelation, "  ")
	sb.WriteByte('}')
}

// writeBlockStatements writes one statement per line at 2-space indent,
// preserving any comment lines interleaved between them.
//
// It walks the block's children in document order rather than the typed
// accessor list, because a comment is a trivia token parked between two
// statement nodes and the typed accessors do not return it. Walking the typed
// list is how the formatter came to delete every comment in a file.
func writeBlockStatements(sb *strings.Builder, node syntax.SyntaxNode, stmtKind syntax.SyntaxKind, indent string) {
	insideBraces := false
	for el := range node.ChildrenIter() {
		switch c := el.(type) {
		case syntax.SyntaxToken:
			if c.Kind() == syntax.SyntaxKindLBrace {
				insideBraces = true
				continue
			}
			// Comments before the `{` were written above the header by the
			// caller; re-emitting them here would both duplicate them and
			// indent them into the block body.
			if insideBraces && isCommentKind(c.Kind()) {
				sb.WriteString(indent)
				sb.WriteString(c.Text())
				sb.WriteByte('\n')
			}
		case syntax.SyntaxNode:
			if c.Kind() != stmtKind {
				continue
			}
			sb.WriteString(indent)
			sb.WriteString(syntax.NodeSourceText(c))
			sb.WriteByte('\n')
		}
	}
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
	depth := 0
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

		// `}` closes its block, so it dedents before it is placed.
		if tok.Kind() == syntax.SyntaxKindRBrace && depth > 0 {
			depth--
		}

		sb.WriteString(separatorFor(prev, gap, tok, depth))
		sb.WriteString(tok.Text())

		if tok.Kind() == syntax.SyntaxKindLBrace {
			depth++
		}

		cur := tok
		prev = &cur
		gap = ""
	}
}

// significantTokens returns a node's leaf tokens with whitespace dropped but
// comments kept.
//
// Tokens() drops line and block comments along with the whitespace, because
// all three are trivia. That is right for a parser that only cares about
// structure and wrong for a renderer, which has to put every byte the author
// wrote back on disk. Using it is why Format Document silently deleted every
// comment in a file.
func significantTokens(node syntax.SyntaxNode) []syntax.SyntaxToken {
	all := node.AllTokens()
	out := make([]syntax.SyntaxToken, 0, len(all))
	for _, tok := range all {
		if tok.Kind() == syntax.SyntaxKindWhitespace {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// writeRefWithComments writes a reference as one atom, with no separator
// between its parts so `olxeu/realestate` stays glued, while keeping any
// comment the parser attached inside it.
//
// A comment can land inside a Ref node whenever the author wrote it between
// the field's `:` or `,` and the value, as in `repo: // note` with the value
// on the next line. RefText() reads Tokens(), which excludes trivia, so the
// atomic-ref path silently ate those comments; and because the atomic path
// runs before the comment branch in formatDecl, the generic handling never got
// a chance either.
//
// The comment keeps its position relative to the value rather than being moved
// after it. Order is part of the contract, and moving it would trip the
// content-drift check for good reason: a reader cannot tell which of two
// values a relocated comment refers to. A line comment runs to end of line, so
// the value that follows it has to go on the next line or the comment would
// swallow it.
func writeRefWithComments(sb *strings.Builder, ref syntax.SyntaxNode, indentDepth int) {
	var parts []syntax.SyntaxToken
	for _, tok := range ref.AllTokens() {
		if tok.Kind() != syntax.SyntaxKindWhitespace {
			parts = append(parts, tok)
		}
	}
	for i, tok := range parts {
		sb.WriteString(tok.Text())
		if isCommentKind(tok.Kind()) && i < len(parts)-1 {
			sb.WriteByte('\n')
			sb.WriteString(strings.Repeat("  ", indentDepth))
		}
	}
}

// formatDecl formats a single top-level declaration into sb.
func formatDecl(sb *strings.Builder, node syntax.SyntaxNode) {
	tokens := significantTokens(node)
	if len(tokens) == 0 {
		return
	}

	depth := 0
	needsNewline := false

	for i, tok := range tokens {
		kind := tok.Kind()

		// A reference is an atom, not a token sequence. Its `/` and `:`
		// separators are leaf tokens, so falling through to the space-joining
		// logic below turns `repo: olxeu/realestate/subscriptions` into
		// `olxeu / realestate / subscriptions` and `bc:re/billing` into
		// `bc: re/billing`, neither of which parses back. Emit the whole ref
		// once, from its own accessor, and skip the tokens it covers.
		//
		// Doing it here rather than per block type covers every declaration
		// formatDecl handles, present and future: service `contexts:`,
		// `catalog_ref:` and `repo:` values, exposure `to:` / `through:` /
		// `contexts:` values, and domain bounded-context lists.
		if parent := tok.Parent(); parent != nil && parent.Kind() == syntax.SyntaxKindRef {
			if i > 0 {
				if prevParent := tokens[i-1].Parent(); prevParent != nil && prevParent.Equal(*parent) {
					continue // covered by the RefText already written
				}
			}
			sep := tokenSeparator(tokens, i, depth, needsNewline)
			needsNewline = false
			sb.WriteString(sep)
			writeRefWithComments(sb, *parent, depth+1)
			continue
		}

		switch {
		case isCommentKind(kind):
			// A comment always owns its line. Placing it inline would let a
			// `// note` swallow whatever the separator logic put after it.
			if i > 0 {
				sb.WriteByte('\n')
				sb.WriteString(strings.Repeat("  ", depth))
			}
			sb.WriteString(tok.Text())
			needsNewline = true
			continue
		}

		switch kind {
		case syntax.SyntaxKindLBrace:
			sb.WriteString(" {")
			depth++
			needsNewline = true

		case syntax.SyntaxKindRBrace:
			// Floor the decrement. An unbalanced `}` only produces a
			// warning-severity diagnostic, so it reaches here with depth 0 and
			// would make strings.Repeat panic with a negative count.
			if depth > 0 {
				depth--
			}
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
