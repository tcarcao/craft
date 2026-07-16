package sema

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// TestGlossaryVerbsSync guards the parser's declared glossary-relation
// vocabulary against silent drift. Unlike context_map's edgeRelationshipVerbs
// (see TestEdgeVerbVocabulariesInSync), sema does not branch on the glossary
// relation verb — resolution and the shape diagnostics in this file (Task
// A3) treat same_as/contrasts/distinct_from identically — so there is no
// sema-side verb map to mirror. This test instead pins down
// syntax.GlossaryVerbs() itself, the single source of truth, so an
// added/removed verb there is caught here rather than silently changing
// behaviour downstream.
func TestGlossaryVerbsSync(t *testing.T) {
	want := []string{"same_as", "contrasts", "distinct_from"}
	got := syntax.GlossaryVerbs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("syntax.GlossaryVerbs() = %v, want %v", got, want)
	}
}
