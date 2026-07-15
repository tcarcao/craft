// Internal (whitebox) test package: classifyEdgeVerb and the edgeRealizationVerbs/
// edgeTermVerbs maps are unexported, so these tests live in package sema rather
// than package sema_test (see validate_test.go for the blackbox validation tests).
package sema

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestClassifyEdgeVerb_UnrecognisedVerb(t *testing.T) {
	// A verb outside both maps is unreachable from .craft source (isEdgeKeyword
	// gates it), so test the sema classifier directly.
	diags := classifyEdgeVerb("f.craft", "bogus_verb", "bc", "service", "bc:x", "service:y", 1, 0)
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/unrecognised-edge-verb" {
		t.Errorf("code = %q, want craft/sema/unrecognised-edge-verb", diags[0].Code)
	}
	if diags[0].Severity != model.SeverityError {
		t.Errorf("severity = %q, want error", diags[0].Severity)
	}
}

func TestEdgeVerbVocabulariesInSync(t *testing.T) {
	parser := map[string]bool{}
	for _, k := range syntax.EdgeKeywords() {
		parser[k] = true
	}
	sema := map[string]bool{}
	for k := range edgeRealizationVerbs {
		sema[k] = true
	}
	for k := range edgeTermVerbs {
		sema[k] = true
	}
	if !reflect.DeepEqual(parser, sema) {
		t.Fatalf("parser EdgeKeywords and sema edge-verb maps disagree:\n parser=%v\n sema=%v", parser, sema)
	}
}
