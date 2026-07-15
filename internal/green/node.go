package green

// GreenElement is either *GreenNode or *GreenToken.
type GreenElement interface {
	Width() TextSize
	isGreenElement()
}

// GreenToken is a position-independent leaf node.
// Width is the byte length of Text.
type GreenToken struct {
	Kind SyntaxKind
	Text string
}

func (t *GreenToken) Width() TextSize { return TextSize(len(t.Text)) }
func (t *GreenToken) isGreenElement() {}

// GreenNode is a position-independent internal node.
// width is cached as the sum of all children's widths.
type GreenNode struct {
	Kind     SyntaxKind
	Children []GreenElement
	width    TextSize
}

func (n *GreenNode) Width() TextSize { return n.width }
func (n *GreenNode) isGreenElement() {}

// NewGreenNode creates a GreenNode and caches the total width.
func NewGreenNode(kind SyntaxKind, children []GreenElement) *GreenNode {
	var w TextSize
	for _, c := range children {
		w += c.Width()
	}
	return &GreenNode{Kind: kind, Children: children, width: w}
}
