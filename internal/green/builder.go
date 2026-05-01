package green

import "reflect"

// Checkpoint records a position in the builder's children accumulator.
// Used with StartNodeAt for the "wrap" pattern.
type Checkpoint int

// BuilderSnapshot captures full builder state for rollback.
// The parser stores TokPos separately (it owns the token position).
type BuilderSnapshot struct {
	ParentsLen  int
	ChildrenLen int
	TokPos      int
}

type parentFrame struct {
	kind          SyntaxKind
	childrenStart int
}

// GreenNodeBuilder accumulates green tree events for a full parse.
// A single instance is reused for the entire file — matches rowan's GreenNodeBuilder.
// StartNode/FinishNode calls must be balanced; Token may appear between them.
type GreenNodeBuilder struct {
	parents  []parentFrame
	children []GreenElement
	cache    *NodeCache // nil disables interning
}

// StartNode opens a new node scope of the given kind.
func (b *GreenNodeBuilder) StartNode(kind SyntaxKind) {
	b.parents = append(b.parents, parentFrame{kind, len(b.children)})
}

// StartNodeAt retroactively opens a node scope from the checkpoint position,
// wrapping all children emitted since the checkpoint. Matches rowan's start_node_at.
func (b *GreenNodeBuilder) StartNodeAt(cp Checkpoint, kind SyntaxKind) {
	b.parents = append(b.parents, parentFrame{kind, int(cp)})
}

// Checkpoint returns the current children count — use with StartNodeAt.
func (b *GreenNodeBuilder) Checkpoint() Checkpoint {
	return Checkpoint(len(b.children))
}

// Token appends a leaf token to the current scope.
// pos is the byte offset of the token start from the beginning of the source file.
func (b *GreenNodeBuilder) Token(kind SyntaxKind, text string, pos TextSize) {
	b.children = append(b.children, &GreenToken{Kind: kind, Text: text, Pos: pos})
}

// FinishNode closes the current scope and appends the resulting GreenNode
// as a child of the enclosing scope.
func (b *GreenNodeBuilder) FinishNode() {
	frame := b.parents[len(b.parents)-1]
	b.parents = b.parents[:len(b.parents)-1]
	children := make([]GreenElement, len(b.children)-frame.childrenStart)
	copy(children, b.children[frame.childrenStart:])
	b.children = b.children[:frame.childrenStart]
	node := NewGreenNode(frame.kind, children)
	if b.cache != nil {
		node = b.cache.Intern(node)
	}
	b.children = append(b.children, node)
}

// Finish returns the root GreenNode. Call after the outermost FinishNode.
// Panics if the builder is in an inconsistent state (missing FinishNode calls).
func (b *GreenNodeBuilder) Finish() *GreenNode {
	return b.children[0].(*GreenNode)
}

// Parents exposes the internal parents slice for snapshot/rollback.
func (b *GreenNodeBuilder) Parents() []parentFrame { return b.parents }

// Children exposes the internal children slice for snapshot/rollback.
func (b *GreenNodeBuilder) Children() []GreenElement { return b.children }

// SetParents replaces the parents slice (used during rollback).
func (b *GreenNodeBuilder) SetParents(p []parentFrame) { b.parents = p }

// SetChildren replaces the children slice (used during rollback).
func (b *GreenNodeBuilder) SetChildren(c []GreenElement) { b.children = c }

// NodeCache interns GreenNodes by structural identity (pointer-based on children,
// bottom-up — correct because children are interned before parents).
// nil-safe: if cache is nil, NewGreenNode is called directly.
type NodeCache struct {
	m map[nodeCacheKey]*GreenNode
}

type nodeCacheKey struct {
	kind SyntaxKind
	hash uint64
}

// Intern returns a deduplicated GreenNode for n. If an identical node (same kind
// and same children pointers) already exists in the cache, it is returned instead.
func (c *NodeCache) Intern(n *GreenNode) *GreenNode {
	if c.m == nil {
		c.m = make(map[nodeCacheKey]*GreenNode)
	}
	key := nodeCacheKey{kind: n.Kind, hash: hashChildren(n.Children)}
	if existing, ok := c.m[key]; ok && childrenEqual(existing.Children, n.Children) {
		return existing
	}
	c.m[key] = n
	return n
}

func hashChildren(children []GreenElement) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range children {
		h ^= uint64(reflect.ValueOf(c).Pointer())
		h *= 1099511628211
	}
	return h
}

func childrenEqual(a, b []GreenElement) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
