package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestProtocolVerbs_Contents(t *testing.T) {
	want := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
		"HEAD": true, "OPTIONS": true, "GRPC": true, "TOPIC": true, "QUERY": true,
	}
	got := syntax.ProtocolVerbs()
	if len(got) != len(want) {
		t.Fatalf("ProtocolVerbs() returned %d verbs, want %d: %v", len(got), len(want), got)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("unexpected verb %q", v)
		}
	}
}

func TestProtocolVerbs_SortedAndStable(t *testing.T) {
	a := syntax.ProtocolVerbs()
	b := syntax.ProtocolVerbs()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("ProtocolVerbs() is not stable: %v vs %v", a, b)
		}
	}
	for i := 1; i < len(a); i++ {
		if a[i-1] >= a[i] {
			t.Errorf("ProtocolVerbs() not sorted at %d: %q then %q", i, a[i-1], a[i])
		}
	}
}

func TestOpAnnotationKind_IsNode(t *testing.T) {
	if !syntax.SyntaxKindOpAnnotation.IsNode() {
		t.Error("SyntaxKindOpAnnotation should be a node kind")
	}
	if syntax.SyntaxKindOpVerb.IsToken() != true {
		t.Error("SyntaxKindOpVerb should be a token kind")
	}
}
