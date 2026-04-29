package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestSyntaxKind_IsToken(t *testing.T) {
	if !syntax.SyntaxKindIdent.IsToken() {
		t.Error("SyntaxKindIdent should be a token kind")
	}
	if syntax.SyntaxKindIdent.IsNode() {
		t.Error("SyntaxKindIdent should not be a node kind")
	}
}

func TestSyntaxKind_IsNode(t *testing.T) {
	if !syntax.SyntaxKindFile.IsNode() {
		t.Error("SyntaxKindFile should be a node kind")
	}
	if syntax.SyntaxKindFile.IsToken() {
		t.Error("SyntaxKindFile should not be a token kind")
	}
}

func TestSyntaxKind_NodeBoundary(t *testing.T) {
	nodeKinds := []syntax.SyntaxKind{
		syntax.SyntaxKindFile, syntax.SyntaxKindActorDecl,
		syntax.SyntaxKindUseCaseDecl, syntax.SyntaxKindErrorNode,
	}
	for _, k := range nodeKinds {
		if int(k) < 1000 {
			t.Errorf("node kind %d is < 1000", k)
		}
	}
}

func TestSyntaxKind_Invalid_IsNeitherTokenNorNode(t *testing.T) {
	if syntax.SyntaxKindInvalid.IsToken() {
		t.Error("SyntaxKindInvalid should not be a token")
	}
	if syntax.SyntaxKindInvalid.IsNode() {
		t.Error("SyntaxKindInvalid should not be a node")
	}
}

