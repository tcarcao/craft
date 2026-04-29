package syntax

// SyntaxElement is either a *SyntaxNode or *SyntaxToken.
type SyntaxElement interface {
	syntaxKind() SyntaxKind
	isSyntaxElement()
}

// SyntaxToken is a leaf in the syntax tree.
type SyntaxToken struct {
	Kind  SyntaxKind
	Value string
	Line  int // 1-based
	Col   int // 1-based
}

func (t *SyntaxToken) Length() int            { return len([]rune(t.Value)) }
func (t *SyntaxToken) syntaxKind() SyntaxKind { return t.Kind }
func (t *SyntaxToken) isSyntaxElement()       {}

// SyntaxNode is an internal node in the syntax tree.
type SyntaxNode struct {
	Kind     SyntaxKind
	Children []SyntaxElement
}

func (n *SyntaxNode) syntaxKind() SyntaxKind { return n.Kind }
func (n *SyntaxNode) isSyntaxElement()       {}

// ChildToken returns the first direct child token matching any of the given kinds.
func (n *SyntaxNode) ChildToken(kinds ...SyntaxKind) *SyntaxToken {
	for _, child := range n.Children {
		tok, ok := child.(*SyntaxToken)
		if !ok {
			continue
		}
		for _, k := range kinds {
			if tok.Kind == k {
				return tok
			}
		}
	}
	return nil
}

// ChildTokens returns all direct child tokens matching any of the given kinds.
func (n *SyntaxNode) ChildTokens(kinds ...SyntaxKind) []*SyntaxToken {
	var result []*SyntaxToken
	for _, child := range n.Children {
		tok, ok := child.(*SyntaxToken)
		if !ok {
			continue
		}
		for _, k := range kinds {
			if tok.Kind == k {
				result = append(result, tok)
				break
			}
		}
	}
	return result
}

// ChildNode returns the first direct child node matching the given kind.
func (n *SyntaxNode) ChildNode(kind SyntaxKind) *SyntaxNode {
	for _, child := range n.Children {
		node, ok := child.(*SyntaxNode)
		if ok && node.Kind == kind {
			return node
		}
	}
	return nil
}

// ChildNodes returns all direct child nodes matching the given kind.
func (n *SyntaxNode) ChildNodes(kind SyntaxKind) []*SyntaxNode {
	var result []*SyntaxNode
	for _, child := range n.Children {
		node, ok := child.(*SyntaxNode)
		if ok && node.Kind == kind {
			result = append(result, node)
		}
	}
	return result
}

// isTrivia reports whether a token kind is trivia (excluded from Tokens()).
func isTrivia(k SyntaxKind) bool {
	return k == SyntaxKindLineComment || k == SyntaxKindBlockComment
}

// Tokens returns all leaf tokens in document order, excluding trivia (comments).
func (n *SyntaxNode) Tokens() []*SyntaxToken {
	return n.collectTokens(false)
}

// AllTokens returns all leaf tokens in document order, including trivia.
func (n *SyntaxNode) AllTokens() []*SyntaxToken {
	return n.collectTokens(true)
}

func (n *SyntaxNode) collectTokens(includeTrivia bool) []*SyntaxToken {
	var result []*SyntaxToken
	for _, child := range n.Children {
		switch c := child.(type) {
		case *SyntaxToken:
			if includeTrivia || !isTrivia(c.Kind) {
				result = append(result, c)
			}
		case *SyntaxNode:
			result = append(result, c.collectTokens(includeTrivia)...)
		}
	}
	return result
}
