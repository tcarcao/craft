package green

// GreenElement is either *GreenNode or *GreenToken.
type GreenElement interface {
	Width() TextSize
	isGreenElement()
}

// GreenToken is a leaf node carrying the token kind, text, and its byte offset
// from the start of the source file. Pos is populated by the parser using the
// lexer token's line/col converted via LineIndex.Offset; it must not be used
// for tree-sharing (nodes are not interned by position).
type GreenToken struct {
	Kind SyntaxKind
	Text string
	Pos  TextSize // byte offset of token start from beginning of source file
}

func (t *GreenToken) Width() TextSize  { return TextSize(len(t.Text)) }
func (t *GreenToken) isGreenElement() {}

// GreenNode is a position-independent internal node.
// width is cached as the sum of all children's widths.
type GreenNode struct {
	Kind     SyntaxKind
	Children []GreenElement
	width    TextSize
}

func (n *GreenNode) Width() TextSize  { return n.width }
func (n *GreenNode) isGreenElement() {}

// NewGreenNode creates a GreenNode and caches the total width.
func NewGreenNode(kind SyntaxKind, children []GreenElement) *GreenNode {
	var w TextSize
	for _, c := range children {
		w += c.Width()
	}
	return &GreenNode{Kind: kind, Children: children, width: w}
}
