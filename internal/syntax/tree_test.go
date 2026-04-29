package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestSyntaxNode_ChildToken(t *testing.T) {
	node := &syntax.SyntaxNode{
		Kind: syntax.SyntaxKindActorDecl,
		Children: []syntax.SyntaxElement{
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindKwActor, Value: "actor", Line: 1, Col: 1},
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindKwUser, Value: "user", Line: 1, Col: 7},
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindIdent, Value: "Alice", Line: 1, Col: 12},
		},
	}

	kwTok := node.ChildToken(syntax.SyntaxKindKwActor)
	if kwTok == nil || kwTok.Value != "actor" {
		t.Errorf("expected actor keyword, got %v", kwTok)
	}

	nameTok := node.ChildToken(syntax.SyntaxKindIdent)
	if nameTok == nil || nameTok.Value != "Alice" {
		t.Errorf("expected name Alice, got %v", nameTok)
	}

	missing := node.ChildToken(syntax.SyntaxKindKwDomain)
	if missing != nil {
		t.Error("expected nil for missing kind")
	}
}

func TestSyntaxNode_ChildToken_MultipleKinds(t *testing.T) {
	node := &syntax.SyntaxNode{
		Kind: syntax.SyntaxKindActorDecl,
		Children: []syntax.SyntaxElement{
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindKwSystem, Value: "system", Line: 1, Col: 7},
		},
	}
	tok := node.ChildToken(syntax.SyntaxKindKwUser, syntax.SyntaxKindKwSystem, syntax.SyntaxKindKwService)
	if tok == nil || tok.Value != "system" {
		t.Errorf("expected system, got %v", tok)
	}
}

func TestSyntaxNode_ChildTokens(t *testing.T) {
	node := &syntax.SyntaxNode{
		Kind: syntax.SyntaxKindActorDecl,
		Children: []syntax.SyntaxElement{
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindIdent, Value: "Alice", Line: 1, Col: 1},
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindIdent, Value: "Bob", Line: 2, Col: 1},
		},
	}
	toks := node.ChildTokens(syntax.SyntaxKindIdent)
	if len(toks) != 2 {
		t.Errorf("expected 2 ident tokens, got %d", len(toks))
	}
}

func TestSyntaxNode_ChildNode(t *testing.T) {
	child := &syntax.SyntaxNode{Kind: syntax.SyntaxKindActorDecl}
	node := &syntax.SyntaxNode{
		Kind:     syntax.SyntaxKindFile,
		Children: []syntax.SyntaxElement{child},
	}
	found := node.ChildNode(syntax.SyntaxKindActorDecl)
	if found != child {
		t.Error("expected to find child actor decl node")
	}
	missing := node.ChildNode(syntax.SyntaxKindDomainDecl)
	if missing != nil {
		t.Error("expected nil for missing child node")
	}
}

func TestSyntaxNode_ChildNodes(t *testing.T) {
	a1 := &syntax.SyntaxNode{Kind: syntax.SyntaxKindActorDecl}
	a2 := &syntax.SyntaxNode{Kind: syntax.SyntaxKindActorDecl}
	d1 := &syntax.SyntaxNode{Kind: syntax.SyntaxKindDomainDecl}
	node := &syntax.SyntaxNode{
		Kind:     syntax.SyntaxKindFile,
		Children: []syntax.SyntaxElement{a1, a2, d1},
	}
	actors := node.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 2 {
		t.Errorf("expected 2 actor nodes, got %d", len(actors))
	}
}

func TestSyntaxNode_Tokens_SkipsComments(t *testing.T) {
	node := &syntax.SyntaxNode{
		Kind: syntax.SyntaxKindActorDecl,
		Children: []syntax.SyntaxElement{
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindLineComment, Value: "// hi", Line: 1, Col: 1},
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindKwActor, Value: "actor", Line: 2, Col: 1},
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindIdent, Value: "Alice", Line: 2, Col: 7},
		},
	}
	toks := node.Tokens()
	if len(toks) != 2 {
		t.Errorf("Tokens() should skip comments, got %d tokens", len(toks))
	}
	if toks[0].Value != "actor" {
		t.Errorf("first token should be 'actor', got %q", toks[0].Value)
	}
}

func TestSyntaxNode_AllTokens_IncludesComments(t *testing.T) {
	node := &syntax.SyntaxNode{
		Kind: syntax.SyntaxKindActorDecl,
		Children: []syntax.SyntaxElement{
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindLineComment, Value: "// hi", Line: 1, Col: 1},
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindKwActor, Value: "actor", Line: 2, Col: 1},
		},
	}
	toks := node.AllTokens()
	if len(toks) != 2 {
		t.Errorf("AllTokens() should include comments, got %d", len(toks))
	}
	if toks[0].Kind != syntax.SyntaxKindLineComment {
		t.Errorf("first token should be comment, got %v", toks[0].Kind)
	}
}

func TestSyntaxNode_Tokens_Recursive(t *testing.T) {
	// Tokens() and AllTokens() must descend into child nodes
	inner := &syntax.SyntaxNode{
		Kind: syntax.SyntaxKindActorDecl,
		Children: []syntax.SyntaxElement{
			&syntax.SyntaxToken{Kind: syntax.SyntaxKindKwActor, Value: "actor", Line: 1, Col: 1},
		},
	}
	outer := &syntax.SyntaxNode{
		Kind:     syntax.SyntaxKindFile,
		Children: []syntax.SyntaxElement{inner},
	}
	toks := outer.Tokens()
	if len(toks) != 1 || toks[0].Value != "actor" {
		t.Errorf("expected recursive token collection, got %v", toks)
	}
}

func TestSyntaxToken_Length(t *testing.T) {
	tok := &syntax.SyntaxToken{Kind: syntax.SyntaxKindIdent, Value: "Alice"}
	if tok.Length() != 5 {
		t.Errorf("expected length 5, got %d", tok.Length())
	}
	// Unicode: length in runes, not bytes
	tok2 := &syntax.SyntaxToken{Kind: syntax.SyntaxKindIdent, Value: "héllo"}
	if tok2.Length() != 5 {
		t.Errorf("expected rune length 5 for unicode, got %d", tok2.Length())
	}
}
