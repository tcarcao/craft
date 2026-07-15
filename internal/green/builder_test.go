package green_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
)

func TestBuilder_SimpleNode(t *testing.T) {
	var b green.GreenNodeBuilder
	b.StartNode(kindFile)
	b.StartNode(kindActorDecl)
	b.Token(kindKwActor, "actor")
	b.Token(kindIdent, " Foo")
	b.FinishNode()
	b.FinishNode()

	root := b.Finish()
	if root.Kind != kindFile {
		t.Errorf("root.Kind = %v, want %v", root.Kind, kindFile)
	}
	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.Children))
	}
	actor := root.Children[0].(*green.GreenNode)
	if actor.Kind != kindActorDecl {
		t.Errorf("actor.Kind = %v, want %v", actor.Kind, kindActorDecl)
	}
	if actor.Width() != 9 { // "actor" + " Foo"
		t.Errorf("actor.Width() = %d, want 9", actor.Width())
	}
}

func TestBuilder_RoundTripText(t *testing.T) {
	src := "actor Foo"
	var b green.GreenNodeBuilder
	b.StartNode(kindFile)
	b.StartNode(kindActorDecl)
	b.Token(kindKwActor, "actor")
	b.Token(kindIdent, " Foo")
	b.FinishNode()
	b.FinishNode()
	root := b.Finish()

	// Reassemble text from leaf tokens
	got := collectText(root)
	if got != src {
		t.Errorf("round-trip text = %q, want %q", got, src)
	}
}

func TestBuilder_Checkpoint_Wrap(t *testing.T) {
	// Parse "Foo" as a token, then wrap it in a node retroactively
	var b green.GreenNodeBuilder
	b.StartNode(kindFile)
	cp := b.Checkpoint()
	b.Token(kindIdent, "Foo")
	b.StartNodeAt(cp, kindActorDecl) // wrap "Foo" retroactively
	b.FinishNode()
	b.FinishNode()

	root := b.Finish()
	actor := root.Children[0].(*green.GreenNode)
	if actor.Kind != kindActorDecl {
		t.Errorf("wrapped node kind = %v, want ActorDecl", actor.Kind)
	}
	if len(actor.Children) != 1 {
		t.Errorf("wrapped node children = %d, want 1", len(actor.Children))
	}
}

func TestBuilder_Snapshot_Rollback(t *testing.T) {
	var b green.GreenNodeBuilder
	var p fakeParser
	p.builder = &b
	p.tokens = []string{"actor", " Foo", " bar"}
	p.pos = 0

	b.StartNode(kindFile)

	// Take snapshot before speculative parse
	snap := green.BuilderSnapshot{ParentsLen: len(b.Parents()), ChildrenLen: len(b.Children()), TokPos: p.pos}

	// Speculatively emit a token then decide to roll back
	b.Token(kindKwActor, p.tokens[p.pos])
	p.pos++

	// Roll back
	b.SetParents(b.Parents()[:snap.ParentsLen])
	b.SetChildren(b.Children()[:snap.ChildrenLen])
	p.pos = snap.TokPos

	// Now emit correctly
	b.Token(kindIdent, "Foo")
	b.FinishNode()
	root := b.Finish()
	if len(root.Children) != 1 {
		t.Errorf("after rollback children = %d, want 1", len(root.Children))
	}
	tok := root.Children[0].(*green.GreenToken)
	if tok.Text != "Foo" {
		t.Errorf("after rollback token = %q, want 'Foo'", tok.Text)
	}
}

// collectText reassembles source text from a green tree.
func collectText(n *green.GreenNode) string {
	var s string
	for _, c := range n.Children {
		switch v := c.(type) {
		case *green.GreenToken:
			s += v.Text
		case *green.GreenNode:
			s += collectText(v)
		}
	}
	return s
}

type fakeParser struct {
	builder *green.GreenNodeBuilder
	tokens  []string
	pos     int
}
