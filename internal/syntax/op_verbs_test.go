package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestOpAnnotationKind_IsNode(t *testing.T) {
	if !syntax.SyntaxKindOpAnnotation.IsNode() {
		t.Error("SyntaxKindOpAnnotation should be a node kind")
	}
	if syntax.SyntaxKindOpVerb.IsToken() != true {
		t.Error("SyntaxKindOpVerb should be a token kind")
	}
}
