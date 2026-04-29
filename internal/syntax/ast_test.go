package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestActorDecl_View(t *testing.T) {
	tree, _, _ := syntax.ParseTree("actor user Alice")
	file := syntax.AsFile(tree)
	actors := file.Actors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	a := actors[0]
	if a.Name() == nil || a.Name().Value != "Alice" {
		t.Errorf("expected name Alice, got %v", a.Name())
	}
	if a.ActorType() == nil || a.ActorType().Kind != syntax.SyntaxKindKwUser {
		t.Errorf("expected user type, got %v", a.ActorType())
	}
	if a.Keyword() == nil || a.Keyword().Line != 1 {
		t.Errorf("expected keyword on line 1, got %v", a.Keyword())
	}
}

func TestActorsBlock_View(t *testing.T) {
	tree, _, _ := syntax.ParseTree("actors { user Alice  system Bob }")
	file := syntax.AsFile(tree)
	actors := file.Actors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 actors from block, got %d", len(actors))
	}
}

func TestDomainDecl_View(t *testing.T) {
	tree, _, _ := syntax.ParseTree("domain Ordering { Cart Checkout }")
	file := syntax.AsFile(tree)
	domains := file.Domains()
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}
	d := domains[0]
	if d.Name() == nil || d.Name().Value != "Ordering" {
		t.Errorf("expected name Ordering, got %v", d.Name())
	}
	bcs := d.BoundedContexts()
	if len(bcs) != 2 {
		t.Errorf("expected 2 bounded contexts, got %d", len(bcs))
	}
	if bcs[0].Name() == nil || bcs[0].Name().Value != "Cart" {
		t.Errorf("expected first BC Cart, got %v", bcs[0].Name())
	}
}

func TestFile_SyntaxTree(t *testing.T) {
	tree, _, _ := syntax.ParseTree("actor user Alice")
	file := syntax.AsFile(tree)
	// AsFile must not panic on nil
	nilFile := syntax.AsFile(nil)
	if len(nilFile.Actors()) != 0 {
		t.Error("nil tree should return empty actors")
	}
	_ = file
}
