package syntax

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
)

// IsSourceComment reports whether tok is a `//@` source cite. A cite stays a
// line comment (trivia); only its text marks it, so every rule for plain
// comments (parser, formatter, completion) applies to it unchanged.
func IsSourceComment(tok SyntaxToken) bool {
	return tok.Kind() == SyntaxKindLineComment && strings.HasPrefix(tok.Text(), lang.SourceCommentPrefix)
}

// SourceComments returns every `//@` token under n, in source order.
func SourceComments(n SyntaxNode) []SyntaxToken {
	var out []SyntaxToken
	for _, tok := range n.AllTokens() {
		if IsSourceComment(tok) {
			out = append(out, tok)
		}
	}
	return out
}

// BodySpan returns the offsets just after the use case's `{` and at its `}`
// (the node's end when it is unclosed). ok is false without a `{`. A comment
// directly above `use_case` is a leading child of the node, so callers that
// want the body only filter by this span.
func (u UseCaseDecl) BodySpan() (start, end green.TextSize, ok bool) {
	lb := u.node.ChildToken(SyntaxKindLBrace)
	if lb == nil {
		return 0, 0, false
	}
	end = u.node.TextRange().End
	if rb := u.node.ChildToken(SyntaxKindRBrace); rb != nil {
		end = rb.Offset()
	}
	return lb.Offset() + green.TextSize(len(lb.Text())), end, true
}

// Node returns the underlying syntax node.
func (u UseCaseDecl) Node() SyntaxNode { return u.node }
