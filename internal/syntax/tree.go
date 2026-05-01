package syntax

import (
	"iter"

	"github.com/tcarcao/craft/internal/green"
)

// SyntaxElement is either SyntaxNode or SyntaxToken.
type SyntaxElement interface {
	Kind() SyntaxKind
	TextRange() green.TextRange
	isSyntaxElement()
}

// SyntaxNode is a red internal node: green node + byte offset from file start.
// Value type — copy is cheap (3 words). Use Equal() instead of == for identity.
type SyntaxNode struct {
	green  *green.GreenNode
	offset green.TextSize
	parent *SyntaxNode // nil for root
}

func (n SyntaxNode) Kind() SyntaxKind { return n.green.Kind }
func (n SyntaxNode) Offset() green.TextSize { return n.offset }
func (n SyntaxNode) TextRange() green.TextRange {
	return green.TextRange{Start: n.offset, End: n.offset + n.green.Width()}
}
func (n SyntaxNode) isSyntaxElement() {}

// Parent returns the enclosing node, or nil for the root.
func (n SyntaxNode) Parent() *SyntaxNode { return n.parent }

// Equal reports whether two nodes refer to the same tree position.
// Use instead of == — parent pointers differ between independently traversed nodes.
func (n SyntaxNode) Equal(other SyntaxNode) bool {
	return n.green == other.green && n.offset == other.offset
}

// SyntaxToken is a red leaf: green token + byte offset from file start.
type SyntaxToken struct {
	green  *green.GreenToken
	offset green.TextSize
	parent *SyntaxNode
}

func (t SyntaxToken) Kind() SyntaxKind { return t.green.Kind }
func (t SyntaxToken) Text() string { return t.green.Text }
func (t SyntaxToken) Offset() green.TextSize { return t.offset }
func (t SyntaxToken) TextRange() green.TextRange {
	return green.TextRange{Start: t.offset, End: t.offset + t.green.Width()}
}
func (t SyntaxToken) isSyntaxElement() {}

// Parent returns the enclosing node.
func (t SyntaxToken) Parent() *SyntaxNode { return t.parent }

// Root wraps a GreenNode as a red root. Zero allocation.
func Root(g *green.GreenNode) SyntaxNode {
	return SyntaxNode{green: g, offset: 0, parent: nil}
}

// ChildrenIter returns a lazy iterator over all direct children with correct offsets.
// Requires Go 1.23.
func (n SyntaxNode) ChildrenIter() iter.Seq[SyntaxElement] {
	return func(yield func(SyntaxElement) bool) {
		offset := n.offset
		for _, gc := range n.green.Children {
			var el SyntaxElement
			switch g := gc.(type) {
			case *green.GreenNode:
				el = SyntaxNode{green: g, offset: offset, parent: &n}
			case *green.GreenToken:
				el = SyntaxToken{green: g, offset: offset, parent: &n}
			}
			offset += gc.Width()
			if !yield(el) {
				return
			}
		}
	}
}

// Children returns all direct children as a slice.
func (n SyntaxNode) Children() []SyntaxElement {
	var result []SyntaxElement
	for el := range n.ChildrenIter() {
		result = append(result, el)
	}
	return result
}

// ChildToken returns the first direct child token matching any of the given kinds.
func (n SyntaxNode) ChildToken(kinds ...SyntaxKind) *SyntaxToken {
	for el := range n.ChildrenIter() {
		tok, ok := el.(SyntaxToken)
		if !ok {
			continue
		}
		for _, k := range kinds {
			if tok.Kind() == k {
				return &tok
			}
		}
	}
	return nil
}

// ChildTokens returns all direct child tokens matching any of the given kinds.
func (n SyntaxNode) ChildTokens(kinds ...SyntaxKind) []SyntaxToken {
	var result []SyntaxToken
	for el := range n.ChildrenIter() {
		tok, ok := el.(SyntaxToken)
		if !ok {
			continue
		}
		for _, k := range kinds {
			if tok.Kind() == k {
				result = append(result, tok)
				break
			}
		}
	}
	return result
}

// ChildNode returns the first direct child node matching the given kind.
func (n SyntaxNode) ChildNode(kind SyntaxKind) *SyntaxNode {
	for el := range n.ChildrenIter() {
		node, ok := el.(SyntaxNode)
		if ok && node.Kind() == kind {
			return &node
		}
	}
	return nil
}

// ChildNodes returns all direct child nodes matching the given kind.
func (n SyntaxNode) ChildNodes(kind SyntaxKind) []SyntaxNode {
	var result []SyntaxNode
	for el := range n.ChildrenIter() {
		node, ok := el.(SyntaxNode)
		if ok && node.Kind() == kind {
			result = append(result, node)
		}
	}
	return result
}

func isTrivia(k SyntaxKind) bool {
	return k == SyntaxKindLineComment || k == SyntaxKindBlockComment
}

// Tokens returns all leaf tokens in document order, excluding trivia.
func (n SyntaxNode) Tokens() []SyntaxToken { return n.collectTokens(false) }

// AllTokens returns all leaf tokens in document order, including trivia.
func (n SyntaxNode) AllTokens() []SyntaxToken { return n.collectTokens(true) }

func (n SyntaxNode) collectTokens(includeTrivia bool) []SyntaxToken {
	var result []SyntaxToken
	for el := range n.ChildrenIter() {
		switch c := el.(type) {
		case SyntaxToken:
			if includeTrivia || !isTrivia(c.Kind()) {
				result = append(result, c)
			}
		case SyntaxNode:
			result = append(result, c.collectTokens(includeTrivia)...)
		}
	}
	return result
}

// TokenAtOffset is the result of a cursor-position query.
type TokenAtOffset interface{ isTokenAtOffset() }

// NoToken means the cursor is on whitespace with no adjacent token.
type NoToken struct{}

// Single means the cursor is clearly inside one token.
type Single struct{ Token SyntaxToken }

// Between means the cursor is on the exact boundary between two tokens.
type Between struct{ Left, Right SyntaxToken }

func (NoToken) isTokenAtOffset()  {}
func (Single) isTokenAtOffset()   {}
func (Between) isTokenAtOffset()  {}

// TokenAtOffset returns the leaf token(s) at the given byte offset.
func (n SyntaxNode) TokenAtOffset(offset green.TextSize) TokenAtOffset {
	return tokenAtOffset(n, offset, nil)
}

func tokenAtOffset(n SyntaxNode, offset green.TextSize, prev *SyntaxToken) TokenAtOffset {
	nodeRange := n.TextRange()
	if !nodeRange.Contains(offset) && offset != nodeRange.End {
		return NoToken{}
	}
	for el := range n.ChildrenIter() {
		r := el.TextRange()
		if offset > r.End {
			if tok, ok := el.(SyntaxToken); ok {
				prev = &tok
			}
			continue
		}
		if offset < r.Start {
			if prev != nil {
				return Between{Left: *prev, Right: mustToken(el)}
			}
			return NoToken{}
		}
		switch c := el.(type) {
		case SyntaxToken:
			if offset == r.End && prev != nil {
				return Between{Left: *prev, Right: c}
			}
			return Single{Token: c}
		case SyntaxNode:
			return tokenAtOffset(c, offset, prev)
		}
	}
	return NoToken{}
}

func mustToken(el SyntaxElement) SyntaxToken {
	if tok, ok := el.(SyntaxToken); ok {
		return tok
	}
	panic("mustToken: expected SyntaxToken")
}

// NodeAt returns the deepest node whose range contains the given offset.
func (n SyntaxNode) NodeAt(offset green.TextSize) *SyntaxNode {
	switch t := n.TokenAtOffset(offset).(type) {
	case Single:
		return t.Token.Parent()
	case Between:
		return t.Left.Parent()
	default:
		return nil
	}
}
