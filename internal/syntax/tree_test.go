package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

// treeFromSrc parses src and returns the wrapped red root.
func treeFromSrc(src string) syntax.SyntaxNode {
	g, _, _ := syntax.Parse(src)
	return syntax.Root(g)
}

func TestSyntaxNode_ChildToken(t *testing.T) {
	tree := treeFromSrc("actor user Alice")
	actorNodes := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorNodes) != 1 {
		t.Fatalf("expected 1 actor node, got %d", len(actorNodes))
	}
	node := actorNodes[0]

	kwTok := node.ChildToken(syntax.SyntaxKindKwActor)
	if kwTok == nil || kwTok.Text() != "actor" {
		t.Errorf("expected actor keyword, got %v", kwTok)
	}

	nameTok := node.ChildToken(syntax.SyntaxKindIdent)
	if nameTok == nil || nameTok.Text() != "Alice" {
		t.Errorf("expected name Alice, got %v", nameTok)
	}

	missing := node.ChildToken(syntax.SyntaxKindKwDomain)
	if missing != nil {
		t.Error("expected nil for missing kind")
	}
}

func TestSyntaxNode_ChildToken_MultipleKinds(t *testing.T) {
	tree := treeFromSrc("actor system Bob")
	actorNodes := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorNodes) != 1 {
		t.Fatalf("expected 1 actor node")
	}
	tok := actorNodes[0].ChildToken(syntax.SyntaxKindKwUser, syntax.SyntaxKindKwSystem, syntax.SyntaxKindKwService)
	if tok == nil || tok.Text() != "system" {
		t.Errorf("expected system, got %v", tok)
	}
}

func TestSyntaxNode_ChildTokens(t *testing.T) {
	// A domain body has two BC nodes; each contains an Ident token. We test
	// ChildTokens at the BC level rather than constructing nodes by hand.
	tree := treeFromSrc("domain D { Cart Checkout }")
	dom := tree.ChildNodes(syntax.SyntaxKindDomainDecl)
	if len(dom) != 1 {
		t.Fatalf("expected 1 domain")
	}
	bcs := dom[0].ChildNodes(syntax.SyntaxKindBoundedContext)
	if len(bcs) != 2 {
		t.Fatalf("expected 2 BCs, got %d", len(bcs))
	}
	idents := bcs[0].ChildTokens(syntax.SyntaxKindIdent)
	if len(idents) != 1 {
		t.Errorf("expected 1 ident token in first BC, got %d", len(idents))
	}
}

func TestSyntaxNode_ChildNode(t *testing.T) {
	tree := treeFromSrc("actor user Alice")
	found := tree.ChildNode(syntax.SyntaxKindActorDecl)
	if found == nil {
		t.Fatal("expected to find ActorDecl child")
	}
	missing := tree.ChildNode(syntax.SyntaxKindDomainDecl)
	if missing != nil {
		t.Error("expected nil for missing child node")
	}
}

func TestSyntaxNode_ChildNodes(t *testing.T) {
	tree := treeFromSrc("actor user Alice\nactor user Bob\ndomain D { Cart }")
	actors := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 2 {
		t.Errorf("expected 2 actor nodes, got %d", len(actors))
	}
}

func TestSyntaxNode_Tokens_SkipsComments(t *testing.T) {
	tree := treeFromSrc("// hi\nactor user Alice")
	actorNodes := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorNodes) != 1 {
		t.Fatalf("expected 1 actor node")
	}
	toks := actorNodes[0].Tokens()
	for _, tok := range toks {
		if tok.Kind() == syntax.SyntaxKindLineComment {
			t.Errorf("Tokens() should skip comments; got comment %q", tok.Text())
		}
	}
}

func TestSyntaxNode_AllTokens_IncludesComments(t *testing.T) {
	tree := treeFromSrc("// hi\nactor user Alice")
	actorNodes := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorNodes) != 1 {
		t.Fatalf("expected 1 actor node")
	}
	toks := actorNodes[0].AllTokens()
	hasComment := false
	for _, tok := range toks {
		if tok.Kind() == syntax.SyntaxKindLineComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Error("AllTokens() should include comments")
	}
}

func TestSyntaxNode_Tokens_Recursive(t *testing.T) {
	// Tokens() must descend into child nodes — the file root collects from
	// the actor decl node beneath it.
	tree := treeFromSrc("actor user Alice")
	toks := tree.Tokens()
	if len(toks) == 0 {
		t.Fatal("expected token recursion to surface the actor keyword")
	}
	found := false
	for _, tok := range toks {
		if tok.Text() == "actor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected `actor` token among recursive tokens, got %v", toks)
	}
}
